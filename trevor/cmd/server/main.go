package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/streadway/amqp"
	trevorpb "trevor/proto/trevor/proto"
	"google.golang.org/grpc"
)

type trevorServer struct {
	trevorpb.UnimplementedCrewServiceServer
	mu          sync.Mutex
	state       trevorpb.StatusReply
	turnsNeeded int
	totalLoot   int32
}

var (
	amqpConn *amqp.Connection
	amqpCh   *amqp.Channel
	amqpURL  string
)

func mustConnectRabbit(url string) (*amqp.Connection, *amqp.Channel) {
	const maxRetries = 30
	const backoff = 2 * time.Second
	for i := 1; i <= maxRetries; i++ {
		conn, err := amqp.Dial(url)
		if err == nil {
			ch, err := conn.Channel()
			if err == nil {
				return conn, ch
			}
			_ = conn.Close()
			log.Printf("[RabbitMQ] error canal: %v (retry %d/%d)", err, i, maxRetries)
		} else {
			log.Printf("[RabbitMQ] no conecta: %v (retry %d/%d)", err, i, maxRetries)
		}
		time.Sleep(backoff)
	}
	log.Fatalf("[RabbitMQ] imposible conectar a %s", url)
	return nil, nil
}

func initRabbit(url string) {
	amqpConn, amqpCh = mustConnectRabbit(url)
	log.Printf("[RabbitMQ] conectado a %s", url)
}

func (s *trevorServer) StartDistraction(ctx context.Context, r *trevorpb.StartDistractionReq) (*trevorpb.Ack, error) {
	s.mu.Lock()
	s.state = trevorpb.StatusReply{State: trevorpb.PhaseState_RUNNING}
	s.turnsNeeded = TurnsNeeded(r.Prob)
	s.mu.Unlock()

	log.Printf("[Trevor] Distracción iniciada, turnos=%d", s.turnsNeeded)

	for t := 1; t <= s.turnsNeeded; t++ {
		time.Sleep(TurnDuration())
		s.mu.Lock()
		s.state.TurnsDone = int32(t)
		s.mu.Unlock()
		log.Printf("[Trevor] Turno %d/%d", t, s.turnsNeeded)
		if t == s.turnsNeeded/2 && RandomPct(10) {
			s.mu.Lock()
			s.state.State = trevorpb.PhaseState_FAIL
			s.state.FailReason = "Trevor se emborrachó"
			s.mu.Unlock()
			log.Printf("[Trevor] Falló la distracción: %s", s.state.FailReason)
			return &trevorpb.Ack{Msg: "Distracción fallida"}, nil
		}
	}
	s.mu.Lock()
	s.state.State = trevorpb.PhaseState_SUCCESS
	s.mu.Unlock()
	log.Printf("[Trevor] Distracción completada con éxito")
	return &trevorpb.Ack{Msg: "Distracción completada"}, nil
}

func (s *trevorServer) QueryStatus(context.Context, *trevorpb.StatusReq) (*trevorpb.StatusReply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := s.state
	return &copy, nil
}

func (s *trevorServer) StartHeist(ctx context.Context, r *trevorpb.StartHeistReq) (*trevorpb.Ack, error) {
	s.mu.Lock()
	s.state = trevorpb.StatusReply{State: trevorpb.PhaseState_RUNNING}
	s.turnsNeeded = TurnsNeeded(r.Prob)
	s.totalLoot = r.BaseLoot
	s.mu.Unlock()

	log.Printf("[Trevor] Iniciando golpe, turnos=%d", s.turnsNeeded)

	starsCh := make(chan int)
	failCh := make(chan string, 1)
	go subscribeLesterStars("Trevor", starsCh)
	go func() {
		for stars := range starsCh {
			switch {
			case stars == 5:
				log.Printf("[Trevor] Habilidad: Furia (límite sube a 7★)")
			case stars >= 7:
				select { case failCh <- "Trevor acumuló demasiadas estrellas": default: }
				return
			}
		}
	}()

	for t := 1; t <= s.turnsNeeded; t++ {
		select {
		case reason := <-failCh:
			s.mu.Lock()
			s.state.State = trevorpb.PhaseState_FAIL
			s.state.TurnsDone = int32(t)
			s.state.FailReason = reason
			s.mu.Unlock()
			publishCrewTurn("trevor", -1)
			log.Printf("[Trevor] Golpe FALLÓ: %s", reason)
			return &trevorpb.Ack{Msg: "Golpe fallido"}, nil
		default:
		}
		publishCrewTurn("trevor", t)
		time.Sleep(TurnDuration())
		s.mu.Lock()
		s.state.TurnsDone = int32(t)
		s.mu.Unlock()
		log.Printf("[Trevor] Turno %d/%d", t, s.turnsNeeded)
	}

	s.mu.Lock()
	s.state.State = trevorpb.PhaseState_SUCCESS
	s.mu.Unlock()
	publishCrewTurn("trevor", -1)
	log.Printf("[Trevor] Golpe completado con éxito. Botín final = %d", s.totalLoot)
	return &trevorpb.Ack{Msg: "Golpe completado con éxito."}, nil
}

func (s *trevorServer) GetLoot(context.Context, *trevorpb.Empty) (*trevorpb.LootReply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &trevorpb.LootReply{TotalLoot: s.totalLoot}, nil
}

func (s *trevorServer) ConfirmPayment(ctx context.Context, p *trevorpb.PaymentReq) (*trevorpb.PaymentReply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	log.Printf("[Trevor] Pago recibido: $%d", p.Amount)
	return &trevorpb.PaymentReply{
		Ok:       true,
		Response: fmt.Sprintf("Pago ok: $%d", p.Amount),
	}, nil
}

func subscribeLesterStars(clientName string, starsCh chan<- int) {
	const exchangeName = "stars_exchange"
	if err := amqpCh.ExchangeDeclare(exchangeName, "fanout", true, false, false, false, nil); err != nil {
		log.Fatalf("Exchange %s: %v", exchangeName, err)
	}
	q, err := amqpCh.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		log.Fatalf("QueueDeclare: %v", err)
	}
	if err := amqpCh.QueueBind(q.Name, "", exchangeName, false, nil); err != nil {
		log.Fatalf("QueueBind: %v", err)
	}
	msgs, err := amqpCh.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Consume: %v", err)
	}
	log.Printf("[%s] Suscrito a estrellas (exchange=%s queue=%s)", clientName, exchangeName, q.Name)
	go func() {
		for d := range msgs {
			stars, err := strconv.Atoi(string(d.Body))
			if err != nil {
				log.Printf("[Trevor] error parseando estrellas: %v", err)
				continue
			}
			if stars == -1 {
				log.Printf("[Trevor] Fin del golpe (señal -1)")
				close(starsCh)
				return
			}
			log.Printf("[%s] Estrellas=%d", clientName, stars)
			starsCh <- stars
		}
	}()
}

func publishCrewTurn(member string, turn int) {
	const exchangeName = "turns_exchange"
	if err := amqpCh.ExchangeDeclare(exchangeName, "fanout", true, false, false, false, nil); err != nil {
		log.Fatalf("Exchange %s: %v", exchangeName, err)
	}
	body := strconv.Itoa(turn)
	if err := amqpCh.Publish(exchangeName, "", false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte(body),
	}); err != nil {
		log.Printf("[%s] publish error: %v", member, err)
	} else {
		log.Printf("[%s] Notificando turno %d a Lester", member, turn)
	}
}

func main() {
	amqpURL = os.Getenv("AMQP_URL")
	if amqpURL == "" {
		amqpURL = "amqp://guest:guest@rabbitmq:5672/"
	}
	log.Printf("[Trevor] AMQP_URL=%s", amqpURL)
	initRabbit(amqpURL)

	port := os.Getenv("TREVOR_PORT")
	if port == "" {
		port = "50053"
	}
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	trevorpb.RegisterCrewServiceServer(grpcServer, &trevorServer{})
	log.Printf("[Trevor] gRPC escuchando en :%s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal(err)
	}
	_ = amqpCh.Close()
	_ = amqpConn.Close()
}

func TurnsNeeded(prob int32) int {
	if prob < 0 {
		prob = 0
	}
	if prob > 200 {
		prob = 200
	}
	return int(200 - prob)
}
func TurnDuration() time.Duration { return 80 * time.Millisecond }
func RandomPct(p int) bool {
	if p <= 0 {
		return false
	}
	if p >= 100 {
		return true
	}
	return rand.Intn(100) < p
}
