NET                  ?= heistnet

RABBIT_IMG           ?= rabbitmq:3.13-management
RABBIT_NAME          ?= rabbitmq
RABBIT_PORT_INT      ?= 5672
RABBIT_MGMT_PORT_INT ?= 15672
# Puertos publicados en el host (ajústalos si 5672/15672 están ocupados)
RABBIT_PORT_EXT      ?= 5673
RABBIT_MGMT_PORT_EXT ?= 15673

AMQP_USER            ?= heist
AMQP_PASS            ?= heist123

LESTER_NAME          ?= lester
LESTER_IMG           ?= lester:lab
LESTER_PORT          ?= 50051

# AMQP URL que usará Lester *dentro* de la red Docker
AMQP_URL := amqp://$(AMQP_USER):$(AMQP_PASS)@$(RABBIT_NAME):$(RABBIT_PORT_INT)/

# Timeout de espera (segundos) para que Rabbit quede OK
RABBIT_WAIT_TIMEOUT  ?= 90

.PHONY: up network rabbit rabbit-wait rabbit-user build run logs logs-lester logs-rabbit ps stop clean env reup

# -------- Orquestación principal --------
up: network rabbit rabbit-wait rabbit-user build run
	@echo "✅ RabbitMQ y Lester arriba."
	@$(MAKE) ps

reup: stop up

# -------- Infra --------
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

# Espera activa sin healthcheck: estado 'running' + ping interno
rabbit-wait:
	@echo "Esperando a RabbitMQ (estado=running)…"
	@until [ "$$(docker inspect -f '{{.State.Status}}' $(RABBIT_NAME) 2>/dev/null)" = "running" ]; do \
	  echo "  ..."; sleep 2; \
	done
	@echo "Haciendo ping con rabbitmq-diagnostics…"
	@i=0; \
	until docker exec $(RABBIT_NAME) rabbitmq-diagnostics -q ping >/dev/null 2>&1 ; do \
	  i=$$((i+2)); \
	  if [ $$i -ge $(RABBIT_WAIT_TIMEOUT) ]; then \
	    echo "TIMEOUT esperando a RabbitMQ"; \
	    docker logs $(RABBIT_NAME) | tail -n 80; \
	    exit 1; \
	  fi; \
	done

# Asegura el usuario/permiso (idempotente)
rabbit-user:
	- docker exec $(RABBIT_NAME) rabbitmqctl add_user $(AMQP_USER) $(AMQP_PASS) 2>/dev/null || true
	- docker exec $(RABBIT_NAME) rabbitmqctl set_permissions -p / $(AMQP_USER) '.*' '.*' '.*'
	@echo "Usuario '$(AMQP_USER)' listo."

# -------- Lester --------
build:
	docker build -t $(LESTER_IMG) ./lester

run:
	- docker rm -f $(LESTER_NAME) >/dev/null 2>&1 || true
	docker run -d --name $(LESTER_NAME) --network $(NET) \
	  -e AMQP_URL='$(AMQP_URL)' \
	  -e LESTER_PORT=$(LESTER_PORT) \
	  -p $(LESTER_PORT):$(LESTER_PORT) \
	  $(LESTER_IMG)

# -------- Utilitarios --------
logs: logs-lester
logs-lester:
	docker logs -f $(LESTER_NAME)

logs-rabbit:
	docker logs -f $(RABBIT_NAME)

ps:
	docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'

env:
	- docker exec $(LESTER_NAME) printenv AMQP_URL || true

stop:
	- docker rm -f $(LESTER_NAME) $(RABBIT_NAME) >/dev/null 2>&1 || true

clean: stop
	- docker rmi $(LESTER_IMG) >/dev/null 2>&1 || true
	- docker network rm $(NET) >/dev/null 2>&1 || true
