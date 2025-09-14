SHELL := /bin/bash

# --- Red y RabbitMQ ---
NET                  ?= heistnet
RABBIT_IMG           ?= rabbitmq:3.13-management
RABBIT_NAME          ?= rabbitmq
RABBIT_PORT_EXT      ?= 5673     # expuesto hacia otras VMs
RABBIT_MGMT_PORT_EXT ?= 15673
AMQP_USER            ?= heist
AMQP_PASS            ?= heist123

# --- Lester ---
LESTER_NAME ?= lester
LESTER_IMG  ?= lester:lab
LESTER_PORT ?= 50051

# URL que usará Lester DENTRO de Docker (usa el hostname 'rabbitmq' y el puerto interno 5672)
AMQP_URL_DOCKER := amqp://$(AMQP_USER):$(AMQP_PASS)@$(RABBIT_NAME):5672/

.PHONY: up network rabbit rabbit-wait lester-build lester-run ps logs logs-lester logs-rabbit stop clean env

# Orquestación completa
up: network rabbit rabbit-wait lester-build lester-run
	@$(MAKE) ps

# Red (idempotente)
network:
	- docker network create $(NET)

# RabbitMQ limpio, con usuario creado al boot
rabbit:
	- docker rm -f -v $(RABBIT_NAME) >/dev/null 2>&1 || true
	- docker volume rm $$(docker volume ls -q | grep -Ei 'rabbit|mq' || true) >/dev/null 2>&1 || true
	docker run -d --name $(RABBIT_NAME) --network $(NET) \
	  -p $(RABBIT_PORT_EXT):5672 -p $(RABBIT_MGMT_PORT_EXT):15672 \
	  -e RABBITMQ_DEFAULT_USER=$(AMQP_USER) \
	  -e RABBITMQ_DEFAULT_PASS=$(AMQP_PASS) \
	  $(RABBIT_IMG)

# Espera con timeout y muestra puertos
rabbit-wait:
	@echo "Esperando a RabbitMQ (timeout 90s)…"
	@set -e; ok=0; \
	for i in $$(seq 1 45); do \
	  if docker exec $(RABBIT_NAME) rabbitmq-diagnostics -q check_running >/dev/null 2>&1; then ok=1; break; fi; \
	  sleep 2; \
	done; \
	if [ $$ok -ne 1 ]; then \
	  echo "RabbitMQ NO arrancó, logs:"; docker logs --tail 200 $(RABBIT_NAME); exit 1; \
	fi; \
	echo "Puertos publicados:"; docker port $(RABBIT_NAME); \
	echo "Usuarios:"; docker exec $(RABBIT_NAME) rabbitmqctl list_users

# Build de Lester (usa ./lester)
lester-build:
	docker build -t $(LESTER_IMG) ./lester

# Run de Lester (en la misma red), con AMQP_URL apuntando a rabbitmq:5672 (interno)
lester-run:
	- docker rm -f $(LESTER_NAME) >/dev/null 2>&1 || true
	docker run -d --name $(LESTER_NAME) --network $(NET) \
	  -e AMQP_URL='$(AMQP_URL_DOCKER)' \
	  -e LESTER_PORT=$(LESTER_PORT) \
	  -p $(LESTER_PORT):$(LESTER_PORT) \
	  $(LESTER_IMG)

ps:
	docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'

logs: logs-lester
logs-lester:
	docker logs -f $(LESTER_NAME)
logs-rabbit:
	docker logs -f $(RABBIT_NAME)

stop:
	- docker rm -f $(LESTER_NAME) $(RABBIT_NAME) >/dev/null 2>&1 || true

clean: stop
	- docker volume rm $$(docker volume ls -q | grep -Ei 'rabbit|mq' || true) >/dev/null 2>&1 || true
	- docker rmi $(LESTER_IMG) >/dev/null 2>&1 || true
	- docker network rm $(NET) >/dev/null 2>&1 || true

env:
	- docker exec $(LESTER_NAME) printenv AMQP_URL || true
