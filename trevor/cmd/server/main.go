package main

import (
	"context"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
	"fmt"

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

var amqpConn *amqp.Connection
var amqpCh *amqp.Channel

func connectRabbit() (*amqp.Connection, *amqp.Channel) {
	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	if err != nil {
		log.Fatalf("No se pudo conectar a RabbitMQ: %v", err)

	}

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("No se pudo abrir el canal: %v", err)
	}

	return conn, ch
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

		if t == s.turnsNeeded/2 && RandomPct(10) { // <-- helper local
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

func (s *trevorServer) QueryStatus(ctx context.Context, _ *trevorpb.StatusReq) (*trevorpb.StatusReply, error) {
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

	go func() {
		subscribeLesterStars("Trevor", starsCh)
	}()

	go func() {
		for stars := range starsCh {
			switch {
			case stars == 5:
				log.Printf("[Trevor] Trevor activa su habilidad especial: Furia (¡Limite de fracaso aumentado a 7 estrellas!)")
			case stars >= 7:
				select {
				case failCh <- "Trevor acumuló demasiadas estrellas":
				default: // evita bloqueo si ya se mandó fail
				}
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
			log.Printf("[Trevor] Falló el golpe: %s", s.state.FailReason)
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

//4
func (s *trevorServer) GetLoot(ctx context.Context, _ *trevorpb.Empty) (*trevorpb.LootReply, error) {
	s.mu.Lock()
    defer s.mu.Unlock()
    return &trevorpb.LootReply{TotalLoot: s.totalLoot}, nil

}

func (s *trevorServer) ConfirmPayment(ctx context.Context, p *trevorpb.PaymentReq) (*trevorpb.PaymentReply, error) {
	s.mu.Lock()
	log.Printf("[Trevor]   Justo lo que esperaba! $%d", p.Amount)
	defer s.mu.Unlock()
	return &trevorpb.PaymentReply{
		Ok:       true,
		Response: fmt.Sprintf(" Justo lo que esperaba! $%d", p.Amount),
	}, nil
}


func subscribeLesterStars(clientName string, starsCh chan<- int) {
	exchangeName := "stars_exchange"
	if err := amqpCh.ExchangeDeclare(exchangeName, "fanout", true, false, false, false, nil); err != nil {
		log.Fatalf("Error declarando exchange %s: %v", exchangeName, err)
	}
	q, err := amqpCh.QueueDeclare(
		"",    //nombre vacío
		false, //durable
		true,  //auto-delete
		true,  //exclusive
		false, //no-wait
		nil,
	)
	if err != nil {
		log.Fatalf("Error declarando cola temporal: %v", err)
	}

	err = amqpCh.QueueBind(
		q.Name,
		"",
		exchangeName,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Error haciendo bind de la cola: %v", err)
	}

	msgs, err := amqpCh.Consume(
		q.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Error consumiendo mensajes: %v", err)
	}

	log.Printf("[%s] Suscrito a estrellas de Lester", clientName)

	go func() {
		for d := range msgs {
			stars, err := strconv.Atoi(string(d.Body))
			if err != nil {
				log.Printf("[Trevor] Error convirtiendo estrellas: %v", err)
				continue
			}
			if stars == -1 {
				log.Printf("[Trevor] Fin del golpe")
				close(starsCh)
				return
			}
			log.Printf("[Trevor] Estado estrellas actual según Lester: %s", d.Body)
			starsCh <- stars
		}
	}()
}

func publishCrewTurn(member string, turn int) {
	exchangeName := "turns_exchange"
	err := amqpCh.ExchangeDeclare(
		exchangeName,
		"fanout",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Error creando exchange %s: %v", exchangeName, err)
	}

	body := strconv.Itoa(turn)
	err = amqpCh.Publish(
		exchangeName,
		"",
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		},
	)
	if err != nil {
		log.Printf("[%s] Error publicando turnos en %s: %v", member, member, err)
	} else {
		log.Printf("[%s] Notificando turno %d a Lester", member, turn)
	}
}

func initRabbit() {
	var _ error
	amqpConn, amqpCh = connectRabbit()
	log.Printf("[RabbitMQ] Conexión y canal inicializados")
}

// main
func main() {
	port := os.Getenv("TREVOR_PORT")
	if port == "" {
		port = "50053"
	}
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	initRabbit()
	grpcServer := grpc.NewServer()
	trevorpb.RegisterCrewServiceServer(grpcServer, &trevorServer{})
	log.Printf("[Trevor] gRPC escuchando en :%s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal(err)
	}
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

func TurnDuration() time.Duration {
	return 80 * time.Millisecond
}

func RandomPct(p int) bool {
	if p <= 0 {
		return false
	}
	if p >= 100 {
		return true
	}
	return rand.Intn(100) < p
}
