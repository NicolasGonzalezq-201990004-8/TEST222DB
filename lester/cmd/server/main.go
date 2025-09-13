package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/streadway/amqp"

	lesterpb "lester/proto/lester/proto"

	"google.golang.org/grpc"
)

type lesterServer struct {
	lesterpb.UnimplementedLesterServiceServer
	mu                sync.Mutex
	rejectCount       int
	currentPoliceRisk int32
	currentBaseLoot   int
	grpcServer        *grpc.Server
}

type MissionData struct {
	PoliceRisk int32
	BaseLoot   int
}

var rabbitConn *amqp.Connection
var rabbitCh *amqp.Channel

var missionCh = make(chan MissionData, 1)

func initRabbit() {
	var _ error
	rabbitConn, rabbitCh = connectRabbit()
	log.Printf("[RabbitMQ] Conexión y canal inicializados")
}

func (s *lesterServer) GetOffer(ctx context.Context, r *lesterpb.OfferRequest) (*lesterpb.OfferReply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	has := rand.Intn(100) < 90
	if !has {
		return &lesterpb.OfferReply{HasOffer: false}, nil
	}

	base := 50000 + rand.Intn(100000)
	pf := int32(30 + rand.Intn(71))
	pt := int32(30 + rand.Intn(71))
	risk := int32(10 + rand.Intn(81))
	s.currentPoliceRisk = risk
	s.currentBaseLoot = base

	return &lesterpb.OfferReply{
		HasOffer:     true,
		BaseLoot:     int32(base),
		ProbFranklin: pf,
		ProbTrevor:   pt,
		PoliceRisk:   risk,
	}, nil
}

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

func (s *lesterServer) NotifyDecision(ctx context.Context, d *lesterpb.Decision) (*lesterpb.Ack, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if d.Accepted {
		s.rejectCount = 0
		missionCh <- MissionData{
			PoliceRisk: s.currentPoliceRisk,
			BaseLoot:   s.currentBaseLoot,
		}
		return &lesterpb.Ack{Msg: "Decisión recibida, cerrando fase de negociación"}, nil
	}
	s.rejectCount++
	if s.rejectCount >= 3 {
		log.Printf("[Lester] 3 rechazos → esperando 10s…")
		time.Sleep(10 * time.Second)
		s.rejectCount = 0
	}

	return &lesterpb.Ack{Msg: "Decisión recibida"}, nil
}

func (s *lesterServer) QueryPoliceRisk(ctx context.Context, _ *lesterpb.RiskReq) (*lesterpb.RiskReply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return &lesterpb.RiskReply{
		PoliceRisk: s.currentPoliceRisk,
	}, nil
}

//4
func (s *lesterServer) ConfirmPayment(ctx context.Context, p *lesterpb.PaymentReq) (*lesterpb.PaymentReply, error) {
	s.mu.Lock()
	log.Printf("[Lester]  Un placer hacer negocios. $%d", p.Amount)
	defer s.mu.Unlock()
	return &lesterpb.PaymentReply{
		Ok:       true,
		Response: fmt.Sprintf("Un placer hacer negocios.  $%d", p.Amount),
	}, nil
}


func starsPublisher(policeRisk int32) {
	err := rabbitCh.ExchangeDeclare(
		"stars_exchange", //nombre cola
		"fanout",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Error creando exchange: %v", err)
	}

	turnsCh := make(chan int, 100)
	go subscribeCrewTurns(turnsCh)

	stars := 0
	freq := 100 - int(policeRisk)
	if freq < 10 {
		freq = 10
	}
	count := 0
	for turn := range turnsCh {
		if turn == -1 {
			close(turnsCh)
			stars = -2
			count = freq - 1
		}
		count++
		if count == freq {
			stars++
			count = 0
			body := strconv.Itoa(stars)
			err = rabbitCh.Publish(
				"stars_exchange",
				"",
				false,
				false,
				amqp.Publishing{
					ContentType: "text/plain",
					Body:        []byte(body),
				},
			)
			if err != nil {
				log.Printf("Error publicando estrellas: %v", err)
			} else if stars != -1 {
				log.Printf("[Lester] Aumento en las estrellas: %d estrellas", stars)
			}

		}
	}

}

func subscribeCrewTurns(turnsCh chan<- int) {
	exchangeName := "turns_exchange"

	err := rabbitCh.ExchangeDeclare(
		exchangeName,
		"fanout",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Error declarando exchange %s: %v", exchangeName, err)
	}

	q, err := rabbitCh.QueueDeclare(
		"turns_queue",
		false,
		true,
		true,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Error declarando cola temporal: %v", err)
	}

	err = rabbitCh.QueueBind(q.Name, "", exchangeName, false, nil)
	if err != nil {
		log.Fatalf("Error haciendo bind de la cola: %v", err)
	}

	msgs, err := rabbitCh.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Error consumiendo mensajes: %v", err)
	}

	log.Printf("[Lester] Esperando inicio del golpe.")

	go func() {
		for d := range msgs {
			body := string(d.Body)
			turn, err := strconv.Atoi(body)
			if err != nil {
				log.Printf("[Lester] Error convirtiendo turno: %v", err)
				continue
			}
			if turn == -1 {
				log.Printf("[Lester] Fin del golpe.")
				close(turnsCh)
				return
			}
			log.Printf("[Lester] Turno %d.", turn)
			turnsCh <- turn

		}
	}()

}

func main() {
	port := "50051"
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	lesterpb.RegisterLesterServiceServer(grpcServer, &lesterServer{grpcServer: grpcServer})
	initRabbit()

	go func() {
		mission := <-missionCh
		log.Printf("[Lester] Misión aceptada con riesgo policial = %d y botín inicial = %d.", mission.PoliceRisk, mission.BaseLoot)
		starsPublisher(mission.PoliceRisk)
	}()
	log.Printf("[Lester] gRPC escuchando en :%s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal(err)
	}
	select {}
}
