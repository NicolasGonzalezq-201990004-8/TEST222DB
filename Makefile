# ==== Parámetros ====
NET                  ?= heistnet

RABBIT_IMG           ?= rabbitmq:3.13-management
RABBIT_NAME          ?= rabbitmq
RABBIT_PORT_EXT      ?= 5673    # puerto HOST para AMQP (mapea al 5672 del contenedor)
RABBIT_MGMT_PORT_EXT ?= 15673   # puerto HOST para UI (mapea al 15672 del contenedor)
AMQP_USER            ?= heist
AMQP_PASS            ?= heist123

LESTER_DIR           ?= ./lester
LESTER_IMG           ?= lester:lab
LESTER_NAME          ?= lester
LESTER_PORT          ?= 50051

# URL que usará Lester DENTRO de la red Docker
AMQP_URL             := amqp://$(AMQP_USER):$(AMQP_PASS)@$(RABBIT_NAME):5672/

.PHONY: up network rabbit rabbit-wait rabbit-user lester-build lester-run ps logs logs-rabbit logs-lester stop clean env

# ==== Orquestación ====
up: network rabbit rabbit-wait rabbit-user lester-build lester-run ps
	@echo "✅ RabbitMQ y Lester arriba"

# ==== Red ====
network:
	- docker network create $(NET)

# ==== RabbitMQ ====
rabbit:
	- docker rm -f $(RABBIT_NAME) >/dev/null 2>&1 || true
	- docker volume rm $$(docker volume ls -q | grep -Ei 'rabbit|mq' || true) >/dev/null 2>&1 || true
	docker run -d --name $(RABBIT_NAME) --network $(NET) \
	 -p $(RABBIT_PORT_EXT):5672 -p $(RABBIT_MGMT_PORT_EXT):15672 \
	 -e RABBITMQ_DEFAULT_USER=$(AMQP_USER) -e RABBITMQ_DEFAULT_PASS=$(AMQP_PASS) \
	 $(RABBIT_IMG)

rabbit-wait:
	@echo "Esperando a RabbitMQ (healthcheck)…"
	@until [ "$$(docker inspect -f '{{.State.Health.Status}}' $(RABBIT_NAME) 2>/dev/null)" = "healthy" ]; do \
	  echo "..."; sleep 2; \
	done
	@echo "RabbitMQ OK."
	@docker port $(RABBIT_NAME)

rabbit-user:  # por si quieres re-aplicar permisos (no hace daño)
	docker exec $(RABBIT_NAME) rabbitmqctl add_user $(AMQP_USER) $(AMQP_PASS) 2>/dev/null || true
	docker exec $(RABBIT_NAME) rabbitmqctl set_permissions -p / $(AMQP_USER) '.*' '.*' '.*'
	@echo "Usuario $(AMQP_USER) listo"

# ==== Lester ====
lester-build:
	docker build --no-cache -t $(LESTER_IMG) $(LESTER_DIR)

lester-run:
	- docker rm -f $(LESTER_NAME) >/dev/null 2>&1 || true
	docker run -d --name $(LESTER_NAME) --network $(NET) \
	  -e AMQP_URL='$(AMQP_URL)' \
	  -e LESTER_PORT=$(LESTER_PORT) \
	  -p $(LESTER_PORT):$(LESTER_PORT) \
	  $(LESTER_IMG)

# ==== Utilidades ====
ps:
	docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'

logs: logs-lester
logs-lester:
	docker logs -f $(LESTER_NAME)
logs-rabbit:
	docker logs -f $(RABBIT_NAME)

env:
	- docker exec $(LESTER_NAME) printenv AMQP_URL || true

stop:
	- docker rm -f $(LESTER_NAME) $(RABBIT_NAME) >/dev/null 2>&1 || true

clean: stop
	- docker rmi $(LESTER_IMG) >/dev/null 2>&1 || true
	- docker network rm $(NET) >/dev/null 2>&1 || true
