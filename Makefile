NET                  ?= heistnet
RABBIT_IMG           ?= rabbitmq:3.13-management
RABBIT_NAME          ?= rabbitmq
RABBIT_PORT_INT      ?= 5672
RABBIT_MGMT_PORT_INT ?= 15672
RABBIT_PORT_EXT      ?= 5673
RABBIT_MGMT_PORT_EXT ?= 15673

AMQP_USER            ?= heist
AMQP_PASS            ?= heist123

LESTER_NAME          ?= lester
LESTER_IMG           ?= lester:lab
LESTER_PORT          ?= 50051

# URL que usa Lester DENTRO de la red Docker
AMQP_URL             := amqp://$(AMQP_USER):$(AMQP_PASS)@$(RABBIT_NAME):$(RABBIT_PORT_INT)/

.PHONY: help up network rabbit rabbit-wait build run logs logs-rabbit ps stop clean env

help:
	@echo "Targets:"
	@echo "  make up          -> red, RabbitMQ, health, build y run de Lester"
	@echo "  make logs        -> logs de Lester"
	@echo "  make logs-rabbit -> logs de RabbitMQ"
	@echo "  make ps          -> contenedores y puertos"
	@echo "  make stop        -> elimina lester y rabbitmq"
	@echo "  make clean       -> stop + borra imagen + red"
	@echo "  make env         -> muestra AMQP_URL dentro de Lester"

up: network rabbit rabbit-wait build run ps
	@echo "OK: RabbitMQ y Lester arriba."

network:
	- docker network create $(NET)

rabbit:
	- docker rm -f $(RABBIT_NAME) >/dev/null 2>&1 || true
	docker run -d --name $(RABBIT_NAME) --network $(NET) \
	  -e RABBITMQ_DEFAULT_USER=$(AMQP_USER) \
	  -e RABBITMQ_DEFAULT_PASS=$(AMQP_PASS) \
	  -p $(RABBIT_PORT_EXT):$(RABBIT_PORT_INT) \
	  -p $(RABBIT_MGMT_PORT_EXT):$(RABBIT_MGMT_PORT_INT) \
	  $(RABBIT_IMG)

rabbit-wait:
	@echo "Esperando a RabbitMQ (healthcheck)…"
	@until [ "$$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' $(RABBIT_NAME) 2>/dev/null)" = "healthy" ]; do \
	  echo "  ..."; sleep 2; \
	done
	@echo "RabbitMQ saludable."
	@docker port $(RABBIT_NAME)

build:
	docker build -t $(LESTER_IMG) ./lester

run:
	- docker rm -f $(LESTER_NAME) >/dev/null 2>&1 || true
	docker run -d --name $(LESTER_NAME) --network $(NET) \
	  -e AMQP_URL='$(AMQP_URL)' \
	  -e LESTER_PORT=$(LESTER_PORT) \
	  -p $(LESTER_PORT):$(LESTER_PORT) \
	  $(LESTER_IMG)

logs:
	docker logs -f $(LESTER_NAME)

logs-rabbit:
	docker logs -f $(RABBIT_NAME)

ps:
	docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
	@echo "AMQP_URL (interno para Lester): $(AMQP_URL)"

env:
	- docker exec $(LESTER_NAME) printenv AMQP_URL || true

stop:
	- docker rm -f $(LESTER_NAME) $(RABBIT_NAME) >/dev/null 2>&1 || true

clean: stop
	- docker rmi $(LESTER_IMG) >/dev/null 2>&1 || true
	- docker network rm $(NET) >/dev/null 2>&1 || true
