NET                 ?= heistnet
RABBIT_IMG          ?= rabbitmq:3.13-management
RABBIT_NAME         ?= rabbitmq
RABBIT_PORT_INT     ?= 5672
RABBIT_MGMT_PORT_INT?= 15672
RABBIT_PORT_EXT     ?= 5673
RABBIT_MGMT_PORT_EXT?= 15673

AMQP_USER           ?= heist
AMQP_PASS           ?= heist123

LESTER_NAME         ?= lester
LESTER_IMG          ?= lester:lab
LESTER_PORT         ?= 50051

# URL que usará Lester DENTRO de la red de Docker
AMQP_URL            := amqp://$(AMQP_USER):$(AMQP_PASS)@$(RABBIT_NAME):$(RABBIT_PORT_INT)/

.PHONY: help up network rabbit rabbit-wait rabbit-user build run logs logs-lester logs-rabbit ps stop clean env

help:
	@echo "Targets:"
	@echo "  make up         -> crea red, levanta RabbitMQ, espera healthy, crea usuario, build y run de Lester"
	@echo "  make logs       -> logs de Lester (follow)"
	@echo "  make logs-rabbit-> logs de RabbitMQ (follow)"
	@echo "  make ps         -> contenedores y puertos"
	@echo "  make stop       -> elimina contenedores (lester y rabbitmq)"
	@echo "  make clean      -> stop + borra imagen de lester y la red"
	@echo "  make env        -> imprime AMQP_URL dentro de lester"

# Orquestación completa
up: network rabbit rabbit-wait rabbit-user build run
	@echo "OK: RabbitMQ y Lester están arriba."
	@$(MAKE) ps

# Red dedicada
network:
	- docker network create $(NET)

# RabbitMQ (management) con puertos publicados
rabbit:
	- docker rm -f $(RABBIT_NAME) >/dev/null 2>&1 || true
	docker run -d --name $(RABBIT_NAME) --network $(NET) \
	  -p $(RABBIT_PORT_EXT):$(RABBIT_PORT_INT) \
	  -p $(RABBIT_MGMT_PORT_EXT):$(RABBIT_MGMT_PORT_INT) \
	  $(RABBIT_IMG)

# Espera activa hasta que el nodo esté "running"
rabbit-wait:
	@echo "Esperando a RabbitMQ…"
	@until docker exec $(RABBIT_NAME) rabbitmq-diagnostics -q check_running >/dev/null 2>&1 ; do \
	  echo "  ..."; sleep 2 ; \
	done
	@echo "RabbitMQ listo."

# Crea/asegura el usuario y permisos
rabbit-user:
	docker exec $(RABBIT_NAME) rabbitmqctl add_user $(AMQP_USER) $(AMQP_PASS) 2>/dev/null || true
	docker exec $(RABBIT_NAME) rabbitmqctl set_permissions -p / $(AMQP_USER) '.*' '.*' '.*'
	@echo "Usuario '$(AMQP_USER)' listo."

# Compila Lester (usa el contexto ./lester)
build:
	docker build --no-cache -t $(LESTER_IMG) ./lester

# Ejecuta Lester con AMQP_URL correcta y puerto expuesto
run:
	- docker rm -f $(LESTER_NAME) >/dev/null 2>&1 || true
	docker run -d --name $(LESTER_NAME) --network $(NET) \
	  -e AMQP_URL='$(AMQP_URL)' \
	  -e LESTER_PORT=$(LESTER_PORT) \
	  -p $(LESTER_PORT):$(LESTER_PORT) \
	  $(LESTER_IMG)

logs: logs-lester
logs-lester:
	docker logs -f $(LESTER_NAME)

logs-rabbit:
	docker logs -f $(RABBIT_NAME)

ps:
	docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'

stop:
	- docker rm -f $(LESTER_NAME) $(RABBIT_NAME) >/dev/null 2>&1 || true

clean: stop
	- docker rmi $(LESTER_IMG) >/dev/null 2>&1 || true
	- docker network rm $(NET)   >/dev/null 2>&1 || true

env:
	- docker exec $(LESTER_NAME) printenv AMQP_URL || true
