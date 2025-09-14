package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	franklinpb "franklin/proto/franklin/proto"
	lesterpb   "lester/proto/lester/proto"
	trevorpb   "trevor/proto/trevor/proto"
)

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" { log.Fatalf("Falta variable de entorno %s", k) }
	return v
}

func dialBlocking(addr string) *grpc.ClientConn {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil { log.Fatalf("No se pudo conectar a %s: %v", addr, err) }
	return conn
}

func writeFail(who, reason string) {
	report := fmt.Sprintf(
		"====== REPORTE FINAL DE LA MISIÓN ======\n"+
			"Resultado: FRACASO\nFase: Distracción\nResponsable: %s\nBotín ganado: $0\nMotivo: %s\n"+
			"======================================\n", who, reason)
	_ = os.WriteFile("Reporte.txt", []byte(report), 0644)
	log.Println("Reporte generado correctamente: Reporte.txt")
}

func main() {
	lAddr := mustEnv("LESTER_ADDR")
	fAddr := mustEnv("FRANKLIN_ADDR")
	tAddr := mustEnv("TREVOR_ADDR")
	log.Printf("[Michael] LESTER=%s FRANKLIN=%s TREVOR=%s", lAddr, fAddr, tAddr)

	lc := lesterpb.NewLesterServiceClient(dialBlocking(lAddr))
	fc := franklinpb.NewCrewServiceClient(dialBlocking(fAddr))
	tc := trevorpb.NewCrewServiceClient(dialBlocking(tAddr))

	ctx := context.Background()

	// ===== Fase 1 =====
	var offer *lesterpb.OfferReply
	for {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		o, err := lc.GetOffer(cctx, &lesterpb.OfferRequest{Team: "Michael"})
		cancel()
		if err != nil || o == nil { log.Printf("GetOffer: %v", err); time.Sleep(time.Second); continue }
		if !o.HasOffer { time.Sleep(time.Second); continue }
		if o.PoliceRisk < 80 {
			_, _ = lc.NotifyDecision(ctx, &lesterpb.Decision{Accepted: true})
			offer = o
			log.Printf("Oferta aceptada: base=%d F=%d T=%d riesgo=%d", o.BaseLoot, o.ProbFranklin, o.ProbTrevor, o.PoliceRisk)
			break
		}
		_, _ = lc.NotifyDecision(ctx, &lesterpb.Decision{Accepted: false})
	}

	// ===== Fase 2 =====
	choice := 0 // 0=Trevor, 1=Franklin
	if offer.ProbFranklin >= offer.ProbTrevor {
		choice = 1
		log.Printf("[Michael] Enviando a Franklin (prob=%d)…", offer.ProbFranklin)
		if _, err := fc.StartDistraction(ctx, &franklinpb.StartDistractionReq{Prob: offer.ProbFranklin}); err != nil {
			log.Fatalf("StartDistraction Franklin: %v", err)
		}
		for {
			cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			st, err := fc.QueryStatus(cctx, &franklinpb.StatusReq{})
			cancel()
			if err != nil || st == nil { time.Sleep(500*time.Millisecond); continue }
			log.Printf("[Michael] Franklin turno=%d estado=%v", st.TurnsDone, st.State)
			if st.State == franklinpb.PhaseState_SUCCESS { break }
			if st.State == franklinpb.PhaseState_FAIL { writeFail("Franklin", st.FailReason); return }
			time.Sleep(500 * time.Millisecond)
		}
	} else {
		log.Printf("[Michael] Enviando a Trevor (prob=%d)…", offer.ProbTrevor)
		if _, err := tc.StartDistraction(ctx, &trevorpb.StartDistractionReq{Prob: offer.ProbTrevor}); err != nil {
			log.Fatalf("StartDistraction Trevor: %v", err)
		}
		for {
			cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			st, err := tc.QueryStatus(cctx, &trevorpb.StatusReq{})
			cancel()
			if err != nil || st == nil { time.Sleep(500*time.Millisecond); continue }
			log.Printf("[Michael] Trevor turno=%d estado=%v", st.TurnsDone, st.State)
			if st.State == trevorpb.PhaseState_SUCCESS { break }
			if st.State == trevorpb.PhaseState_FAIL { writeFail("Trevor", st.FailReason); return }
			time.Sleep(500 * time.Millisecond)
		}
	}

	// ===== Fase 3 =====
	var total int32
	if choice == 1 { // Franklin
		ack, err := fc.StartHeist(ctx, &franklinpb.StartHeistReq{
			Prob:       offer.ProbFranklin,
			PoliceRisk: offer.PoliceRisk,
			BaseLoot:   offer.BaseLoot,
		})
		if err != nil || ack == nil { log.Fatalf("StartHeist Franklin: %v", err) }
		log.Printf("[Michael] Golpe finalizado por Franklin: %s", ack.Msg)
		rep, err := fc.GetLoot(ctx, &franklinpb.Empty{})
		if err != nil || rep == nil { log.Fatalf("GetLoot Franklin: %v", err) }
		total = rep.TotalLoot
	} else { // Trevor
		ack, err := tc.StartHeist(ctx, &trevorpb.StartHeistReq{
			Prob:       offer.ProbTrevor,
			PoliceRisk: offer.PoliceRisk,
			BaseLoot:   offer.BaseLoot,
		})
		if err != nil || ack == nil { log.Fatalf("StartHeist Trevor: %v", err) }
		log.Printf("[Michael] Golpe finalizado por Trevor: %s", ack.Msg)
		rep, err := tc.GetLoot(ctx, &trevorpb.Empty{})
		if err != nil || rep == nil { log.Fatalf("GetLoot Trevor: %v", err) }
		total = rep.TotalLoot
	}

	// ===== Fase 4 =====
	share := total / 4
	rem := total % 4
	michaelShare := share
	franklinShare := share
	trevorShare := share
	lesterShare := share + rem

	_, _ = fc.ConfirmPayment(ctx, &franklinpb.PaymentReq{Amount: franklinShare})
	_, _ = tc.ConfirmPayment(ctx, &trevorpb.PaymentReq{Amount: trevorShare})
	_, _ = lc.ConfirmPayment(ctx, &lesterpb.PaymentReq{Amount: lesterShare})

	report := fmt.Sprintf(
		"====== REPORTE FINAL DE LA MISIÓN ======\n"+
			"Resultado: %v\nBotín total: $%d\n\nReparto:\n"+
			" - Michael:  $%d\n - Franklin: $%d\n - Trevor:   $%d\n - Lester:   $%d\n"+
			"======================================\n",
		total > 0, total, michaelShare, franklinShare, trevorShare, lesterShare,
	)
	_ = os.WriteFile("Reporte.txt", []byte(report), 0644)
	log.Println("Reporte generado correctamente: Reporte.txt")
}
