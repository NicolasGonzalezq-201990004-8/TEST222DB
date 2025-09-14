import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	franklinpb "franklin/proto/franklin/proto"
	lesterpb "lester/proto/lester/proto"
	trevorpb "trevor/proto/trevor/proto"

	"google.golang.org/grpc"
	lesterpb   "lester/proto/lester/proto"
	trevorpb   "trevor/proto/trevor/proto"
)

func mustDialEnv(envName, def string) *grpc.ClientConn {
	addr := os.Getenv(envName)
	if addr == "" {
		addr = def
func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("Falta variable de entorno %s", k)
	}
	cc, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	return v
}

func dialBlocking(addr string) *grpc.ClientConn {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		log.Fatalf("dial %s: %v", addr, err)
		log.Fatalf("No se pudo conectar a %s: %v", addr, err)
	}
	return cc
	return conn
}

func main() {
	lcc := mustDialEnv("LESTER_ADDR", "localhost:50051")
	defer lcc.Close()
	fcc := mustDialEnv("FRANKLIN_ADDR", "localhost:50052")
	defer fcc.Close()
	tcc := mustDialEnv("TREVOR_ADDR", "localhost:50053")
	defer tcc.Close()
	lAddr := mustEnv("LESTER_ADDR")
	fAddr := mustEnv("FRANKLIN_ADDR")
	tAddr := mustEnv("TREVOR_ADDR")
	log.Printf("[Michael] LESTER=%s FRANKLIN=%s TREVOR=%s", lAddr, fAddr, tAddr)

	lester := lesterpb.NewLesterServiceClient(lcc)
	franklin := franklinpb.NewCrewServiceClient(fcc)
	trevor := trevorpb.NewCrewServiceClient(tcc)
	lc := lesterpb.NewLesterServiceClient(dialBlocking(lAddr))
	fc := franklinpb.NewCrewServiceClient(dialBlocking(fAddr))
	tc := trevorpb.NewCrewServiceClient(dialBlocking(tAddr))

	ctx := context.Background()
	choice := 0 // 0=trevor 1=franklin

	// ===== Fase 1: Negociación =====
	var offer *lesterpb.OfferReply

	// Fase 1
	for {
		o, err := lester.GetOffer(ctx, &lesterpb.OfferRequest{Team: "Michael"})
		if err != nil {
			log.Fatalf("GetOffer: %v", err)
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		o, err := lc.GetOffer(cctx, &lesterpb.OfferRequest{Team: "Michael"})
		cancel()
		if err != nil || o == nil {
			log.Printf("GetOffer error: %v", err)
			time.Sleep(time.Second)
			continue
		}
		if !o.HasOffer {
			log.Println("No hay oferta, reintentando...")
			time.Sleep(1 * time.Second)
			log.Println("Sin oferta, reintentando…")
			time.Sleep(time.Second)
			continue
		}

		if o.PoliceRisk < 80 {
			_, _ = lester.NotifyDecision(ctx, &lesterpb.Decision{Accepted: true})
			if _, err := lc.NotifyDecision(ctx, &lesterpb.Decision{Accepted: true}); err != nil {
				log.Printf("NotifyDecision: %v", err)
			}
			offer = o
			log.Printf("Oferta aceptada: base=%d F=%d T=%d riesgo=%d",
				o.BaseLoot, o.ProbFranklin, o.ProbTrevor, o.PoliceRisk)
			break
		} else {
			log.Printf("Oferta rechazada: riesgo=%d", o.PoliceRisk)
			_, _ = lester.NotifyDecision(ctx, &lesterpb.Decision{Accepted: false})
		}
		log.Printf("Oferta rechazada: riesgo=%d", o.PoliceRisk)
		_, _ = lc.NotifyDecision(ctx, &lesterpb.Decision{Accepted: false})
	}

	log.Printf("Fase 1 completada, oferta final: %+v", offer)

	// Fase 2
	// ===== Fase 2: Distracción =====
	choice := 0 // 0=Trevor, 1=Franklin
	if offer.ProbFranklin >= offer.ProbTrevor {
		choice = 1
		log.Printf("[Michael] Enviando a Franklin (prob=%d)...", offer.ProbFranklin)
		_, _ = franklin.StartDistraction(ctx, &franklinpb.StartDistractionReq{Prob: offer.ProbFranklin})
		log.Printf("[Michael] Enviando a Franklin (prob=%d)…", offer.ProbFranklin)
		if _, err := fc.StartDistraction(ctx, &franklinpb.StartDistractionReq{Prob: offer.ProbFranklin}); err != nil {
			log.Fatalf("StartDistraction Franklin: %v", err)
		}
		for {
			st, _ := franklin.QueryStatus(ctx, &franklinpb.StatusReq{})
			cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			st, err := fc.QueryStatus(cctx, &franklinpb.StatusReq{})
			cancel()
			if err != nil || st == nil {
				log.Printf("QueryStatus Franklin: %v", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}
			log.Printf("[Michael] Franklin turno=%d estado=%v", st.TurnsDone, st.State)
			if st.State == franklinpb.PhaseState_SUCCESS {
				log.Println("[Michael] Distracción exitosa por Franklin")
				break
			}
			if st.State == franklinpb.PhaseState_FAIL {
				log.Printf("[Michael] Distracción fallida por Franklin (%s)", st.FailReason)
				reporte := fmt.Sprintf(
					"====== REPORTE FINAL DE LA MISIÓN ======\n"+
						"Resultado: FRACASO\n"+
						"Fase: Distracción\n"+
						"Responsable: Franklin\n"+
						"Botín ganado: $0\n"+
						"Motivo: %s\n"+
						"======================================\n",
					st.FailReason,
				)
				err := os.WriteFile("Reporte.txt", []byte(reporte), 0644)
				if err != nil {
					log.Fatalf("Error al escribir el reporte: %v", err)
				}
				log.Println("Reporte generado correctamente: Reporte.txt")
				//log.Println(reporte)
				writeFail("Franklin", st.FailReason)
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	} else {
		log.Printf("[Michael] Enviando a Trevor (prob=%d)...", offer.ProbTrevor)
		_, _ = trevor.StartDistraction(ctx, &trevorpb.StartDistractionReq{Prob: offer.ProbTrevor})
		log.Printf("[Michael] Enviando a Trevor (prob=%d)…", offer.ProbTrevor)
		if _, err := tc.StartDistraction(ctx, &trevorpb.StartDistractionReq{Prob: offer.ProbTrevor}); err != nil {
			log.Fatalf("StartDistraction Trevor: %v", err)
		}
		for {
			st, _ := trevor.QueryStatus(ctx, &trevorpb.StatusReq{})
			cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			st, err := tc.QueryStatus(cctx, &trevorpb.StatusReq{})
			cancel()
			if err != nil || st == nil {
				log.Printf("QueryStatus Trevor: %v", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}
			log.Printf("[Michael] Trevor turno=%d estado=%v", st.TurnsDone, st.State)
			if st.State == trevorpb.PhaseState_SUCCESS {
				log.Println("[Michael] Distracción exitosa por Trevor")
				break
			}
			if st.State == trevorpb.PhaseState_FAIL {
				log.Printf("[Michael] Distracción fallida por Trevor (%s)", st.FailReason)
				reporte := fmt.Sprintf(
					"====== REPORTE FINAL DE LA MISIÓN ======\n"+
						"Resultado: FRACASO\n"+
						"Fase: Distracción\n"+
						"Responsable: Trevor\n"+
						"Botín ganado: $0\n"+
						"Motivo: %s\n"+
						"======================================\n",
					st.FailReason,
				)
				err := os.WriteFile("Reporte.txt", []byte(reporte), 0644)
				if err != nil {
					log.Fatalf("Error al escribir el reporte: %v", err)
				}
				log.Println("Reporte generado correctamente: Reporte.txt")
				//log.Println(reporte)
				writeFail("Trevor", st.FailReason)
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	//fase3
	var Ackf *franklinpb.Ack
	var Ackt *trevorpb.Ack
	if choice == 0 {
		Ackf, _ = franklin.StartHeist(ctx, &franklinpb.StartHeistReq{Prob: offer.ProbFranklin, PoliceRisk: offer.PoliceRisk, BaseLoot: offer.BaseLoot})
		log.Printf("[Michael] Golpe finalizado por Franklin: %s", Ackf.Msg)
		//montoExtra := Ack.ExtraLoot //monto extra ganado por franklin
	} else {
		Ackt, _ = trevor.StartHeist(ctx, &trevorpb.StartHeistReq{Prob: offer.ProbTrevor, PoliceRisk: offer.PoliceRisk, BaseLoot: offer.BaseLoot})
		log.Printf("[Michael] Golpe finalizado por Trevor: %s", Ackt.Msg)
	}

	// ===== Fase 3: Golpe =====
	var total int32
	if choice == 1 { // Franklin
		ack, err := fc.StartHeist(ctx, &franklinpb.StartHeistReq{
			Prob:       offer.ProbFranklin,
			PoliceRisk: offer.PoliceRisk,
			BaseLoot:   offer.BaseLoot,
		})
		if err != nil || ack == nil {
			log.Fatalf("StartHeist Franklin: %v", err)
		}
		log.Printf("[Michael] Golpe finalizado por Franklin: %s", ack.Msg)

	//fase4
	var totalLoot int32 = 0
	if choice == 0 { // Franklin fue al golpe
		st, _ := franklin.GetLoot(ctx, &franklinpb.Empty{})
		totalLoot = st.TotalLoot
	} else { // Trevor fue al golpe
		st, _ := trevor.GetLoot(ctx, &trevorpb.Empty{})
		totalLoot = st.TotalLoot
		rep, err := fc.GetLoot(ctx, &franklinpb.Empty{})
		if err != nil || rep == nil {
			log.Fatalf("GetLoot Franklin: %v", err)
		}
		total = rep.TotalLoot
	} else { // Trevor
		ack, err := tc.StartHeist(ctx, &trevorpb.StartHeistReq{
			Prob:       offer.ProbTrevor,
			PoliceRisk: offer.PoliceRisk,
			BaseLoot:   offer.BaseLoot,
		})
		if err != nil || ack == nil {
			log.Fatalf("StartHeist Trevor: %v", err)
		}
		log.Printf("[Michael] Golpe finalizado por Trevor: %s", ack.Msg)

		rep, err := tc.GetLoot(ctx, &trevorpb.Empty{})
		if err != nil || rep == nil {
			log.Fatalf("GetLoot Trevor: %v", err)
		}
		total = rep.TotalLoot
	}
	share := totalLoot / 4
	remainder := totalLoot % 4

	// ===== Fase 4: Reparto y pagos =====
	share := total / 4
	rem := total % 4
	michaelShare := share
	franklinShare := share
	trevorShare := share
	lesterShare := share + remainder
	lesterShare := share + rem

	if _, err := fc.ConfirmPayment(ctx, &franklinpb.PaymentReq{Amount: franklinShare}); err != nil {
		log.Printf("Pago Franklin: %v", err)
	}
	if _, err := tc.ConfirmPayment(ctx, &trevorpb.PaymentReq{Amount: trevorShare}); err != nil {
		log.Printf("Pago Trevor: %v", err)
	}
	if _, err := lc.ConfirmPayment(ctx, &lesterpb.PaymentReq{Amount: lesterShare}); err != nil {
		log.Printf("Pago Lester: %v", err)
	}

	reporte := fmt.Sprintf(
	report := fmt.Sprintf(
		"====== REPORTE FINAL DE LA MISIÓN ======\n"+
			"Resultado: %v\n"+
			"Botín total: $%d\n\n"+
			"Reparto:\n"+
			" - Michael:  $%d\n"+
			" - Franklin: $%d\n"+
			" - Trevor:   $%d\n"+
			" - Lester:   $%d\n"+
			" - Michael:  $%d\n - Franklin: $%d\n - Trevor:   $%d\n - Lester:   $%d\n"+
			"======================================\n",
		totalLoot > 0, totalLoot, michaelShare, franklinShare, trevorShare, lesterShare,
		total > 0, total, michaelShare, franklinShare, trevorShare, lesterShare,
	)
	_, err := franklin.ConfirmPayment(ctx, &franklinpb.PaymentReq{Amount: franklinShare})
	if err != nil {
		log.Fatalf("Error al pagar a Franklin: %v", err)
	}
	_, err = trevor.ConfirmPayment(ctx, &trevorpb.PaymentReq{Amount: trevorShare})
	if err != nil {
		log.Fatalf("Error al pagar a Trevor: %v", err)
	}
	_, err = lester.ConfirmPayment(ctx, &lesterpb.PaymentReq{Amount: lesterShare})
	if err != nil {
		log.Fatalf("Error al pagar a Lester: %v", err)
	if err := os.WriteFile("Reporte.txt", []byte(report), 0644); err != nil {
		log.Printf("No pude escribir Reporte.txt: %v", err)
	} else {
		log.Println("Reporte generado correctamente: Reporte.txt")
	}
	log.Printf("Michael recibió $%d para sí mismo", michaelShare)

}

	err = os.WriteFile("Reporte.txt", []byte(reporte), 0644)
	if err != nil {
		log.Fatalf("Error al escribir el reporte: %v", err)
	}
func writeFail(who, reason string) {
	report := fmt.Sprintf(
		"====== REPORTE FINAL DE LA MISIÓN ======\n"+
			"Resultado: FRACASO\nFase: Distracción\nResponsable: %s\nBotín ganado: $0\nMotivo: %s\n"+
			"======================================\n", who, reason)
	_ = os.WriteFile("Reporte.txt", []byte(report), 0644)
	log.Println("Reporte generado correctamente: Reporte.txt")
	//log.Println(reporte)

}
