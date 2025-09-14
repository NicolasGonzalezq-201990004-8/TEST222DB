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
ERLANG_COOKIE        ?= COOKIE123

AMQP_URL := amqp://$(AMQP_USER):$(AMQP_PASS)@$(RABBIT_NAME):$(RABBIT_PORT_INT)/

.PHONY: up network rabbit rabbit-wait build run logs logs-rabbit ps stop clean env

up: network rabbit rabbit-wait build run

network:
	- docker network create $(NET)

rabbit:
	- docker rm -f -v $(RABBIT_NAME) >/dev/null 2>&1 || true
	# opcional: limpia volúmenes huérfanos etiquetados
	- docker volume rm $$(docker volume ls -q | grep -i rabbit) >/dev/null 2>&1 || true
	docker run -d --name $(RABBIT_NAME) --network $(NET) \
	  -p $(RABBIT_PORT_EXT):$(RABBIT_PORT_INT) \
	  -p $(RABBIT_MGMT_PORT_EXT):$(RABBIT_MGMT_PORT_INT) \
	  -e RABBITMQ_NODENAME=rabbit@$(RABBIT_NAME) \
	  -e RABBITMQ_ERLANG_COOKIE=$(ERLANG_COOKIE) \
	  -e RABBITMQ_DEFAULT_USER=$(AMQP_USER) \
	  -e RABBITMQ_DEFAULT_PASS=$(AMQP_PASS) \
	  $(RABBIT_IMG)

rabbit-wait:
	@echo "Esperando a RabbitMQ (healthcheck)…"
	@until docker exec $(RABBIT_NAME) rabbitmq-diagnostics -q ping >/dev/null 2>&1 ; do \
	  echo " ..."; sleep 2 ; \
	done
	@echo "RabbitMQ OK."

build:
	docker build -t $(LESTER_IMG) ./lester

run:
	- docker rm -f $(LESTER_NAME) >/dev/null 2>&1 || true
	docker run -d --name $(LESTER_NAME) --network $(NET) \
	  -e AMQP_URL='$(AMQP_URL)' \
	  -e LESTER_PORT=$(LESTER_PORT) \
	  -p $(LESTER_PORT):$(LESTER_PORT) \
	  $(LESTER_IMG)

logs: ; docker logs -f $(LESTER_NAME)
logs-rabbit: ; docker logs -f $(RABBIT_NAME)
ps: ; docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
stop: ; - docker rm -f $(LESTER_NAME) $(RABBIT_NAME) >/dev/null 2>&1 || true
clean: stop ; - docker network rm $(NET) >/dev/null 2>&1 || true
env: ; - docker exec $(LESTER_NAME) printenv AMQP_URL || true
