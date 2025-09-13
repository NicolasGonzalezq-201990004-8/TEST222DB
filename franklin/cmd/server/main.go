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

	franklinpb "franklin/proto/franklin/proto"

	"google.golang.org/grpc"
)

type franklinServer struct {
	franklinpb.UnimplementedCrewServiceServer
	mu          sync.Mutex
	state       franklinpb.StatusReply
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

func (s *franklinServer) StartDistraction(ctx context.Context, r *franklinpb.StartDistractionReq) (*franklinpb.Ack, error) {
	s.mu.Lock()
	s.state = franklinpb.StatusReply{State: franklinpb.PhaseState_RUNNING}
	s.turnsNeeded = TurnsNeeded(r.Prob)
	s.mu.Unlock()

	log.Printf("[Franklin] Distracción iniciada, turnos=%d", s.turnsNeeded)

	for t := 1; t <= s.turnsNeeded; t++ {
		time.Sleep(TurnDuration())

		s.mu.Lock()
		s.state.TurnsDone = int32(t)
		s.mu.Unlock()

		log.Printf("[Franklin] Turno %d/%d", t, s.turnsNeeded)

		// evento a la mitad (10% de fallar)
		if t == s.turnsNeeded/2 && RandomPct(10) {
			s.mu.Lock()
			s.state.State = franklinpb.PhaseState_FAIL
			s.state.FailReason = "Chop ladró"
			s.mu.Unlock()
			log.Printf("[Franklin] Falló la distracción: %s", s.state.FailReason)
			return &franklinpb.Ack{Msg: "Distracción fallida"}, nil
		}
	}

	s.mu.Lock()
	s.state.State = franklinpb.PhaseState_SUCCESS
	s.mu.Unlock()
	log.Printf("[Franklin] Distracción completada con éxito")
	return &franklinpb.Ack{Msg: "Distracción completada"}, nil
}

func (s *franklinServer) QueryStatus(ctx context.Context, _ *franklinpb.StatusReq) (*franklinpb.StatusReply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := s.state
	return &copy, nil
}

func (s *franklinServer) StartHeist(ctx context.Context, r *franklinpb.StartHeistReq) (*franklinpb.Ack, error) {
	s.mu.Lock()
	s.state = franklinpb.StatusReply{State: franklinpb.PhaseState_RUNNING}
	s.turnsNeeded = TurnsNeeded(r.Prob)
	s.totalLoot = r.BaseLoot
	s.mu.Unlock()
	log.Printf("[Franklin] Iniciando golpe, turnos=%d", s.turnsNeeded)

	starsCh := make(chan int)
	var habilidad bool
	failCh := make(chan string, 1)
	go func() {
		subscribeLesterStars("Franklin", starsCh)
	}()
	go func() {
		for stars := range starsCh {
			switch {
			case stars == 3:
				if !habilidad {
					log.Printf("[Franklin] franklin activa su habilidad especial: Chop (¡Cada turno siguiente añadirá $1000 al botín!)")
					habilidad = true
				}
			case stars >= 5:
				select {
				case failCh <- "Franklin acumuló demasiadas estrellas":
				default:
				}
				return
			}
		}
	}()

	for t := 1; t <= s.turnsNeeded; t++ {
		select {
		case reason := <-failCh:
			s.mu.Lock()
			s.state.State = franklinpb.PhaseState_FAIL
			s.state.TurnsDone = int32(t)
			s.state.FailReason = reason
			s.mu.Unlock()
			publishCrewTurn("franklin", -1)
			log.Printf("[Franklin] Falló el golpe: %s", s.state.FailReason)
			return &franklinpb.Ack{Msg: "Golpe fallido"}, nil
		default:
		}
		publishCrewTurn("franklin", t)
		if habilidad {
			s.totalLoot += 1000
			s.mu.Lock()
			s.state.ExtraLoot += 1000
			s.mu.Unlock()
			log.Printf("[Franklin] Chop recuperó $1000, totalLoot=$%d", s.totalLoot)
		}
		time.Sleep(TurnDuration())
		s.mu.Lock()
		s.state.TurnsDone = int32(t)
		s.mu.Unlock()
		log.Printf("[Franklin] Turno %d/%d", t, s.turnsNeeded)
	}
	s.mu.Lock()
	s.state.State = franklinpb.PhaseState_SUCCESS
	s.mu.Unlock()
	publishCrewTurn("franklin", -1)
	log.Printf("[Franklin] Golpe completado con éxito. Botín final = %d", s.totalLoot)
	return &franklinpb.Ack{
		Msg:       "Golpe completado con éxito",
		ExtraLoot: s.state.ExtraLoot,
	}, nil
}


//4
func (s *franklinServer) GetLoot(ctx context.Context, _ *franklinpb.Empty) (*franklinpb.LootReply, error) {
	s.mu.Lock()
    defer s.mu.Unlock()
    return &franklinpb.LootReply{TotalLoot: s.totalLoot}, nil

}

func (s *franklinServer) ConfirmPayment(ctx context.Context, p *franklinpb.PaymentReq) (*franklinpb.PaymentReply, error) {
	s.mu.Lock()
	log.Printf("[Franklin]  Excelente! El pago es correcto. $%d", p.Amount)
	defer s.mu.Unlock()
	return &franklinpb.PaymentReply{
		Ok:       true,
		Response: fmt.Sprintf(" Excelente ! El pago es correcto. $%d", p.Amount),
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
				log.Printf("[Franklin] Fin del golpe")
				close(starsCh)
				return
			}
			log.Printf("[%s] Estado estrellas actual según Lester: %s", clientName, d.Body)
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

func main() {
	port := os.Getenv("FRANKLIN_PORT")
	if port == "" {
		port = "50052"
	}
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	initRabbit()
	grpcServer := grpc.NewServer()
	franklinpb.RegisterCrewServiceServer(grpcServer, &franklinServer{})
	log.Printf("[Franklin] gRPC escuchando en :%s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal(err)
	}

	//cerramos los canales y conexiones RabbitMQ
	amqpCh.Close()
	amqpConn.Close()

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
