SHELL := /bin/bash
NET                  ?= heistnet
RABBIT_IMG           ?= rabbitmq:3.13-management
RABBIT_NAME          ?= rabbitmq
RABBIT_PORT_EXT      ?= 5673
RABBIT_MGMT_PORT_EXT ?= 15673
AMQP_USER            ?= heist
AMQP_PASS            ?= heist123

.PHONY: up network rabbit rabbit-wait logs logs-rabbit ps stop clean

up: network rabbit rabbit-wait
	@$(MAKE) ps

network:
	- docker network create $(NET)

rabbit:
	- docker rm -f -v $(RABBIT_NAME) >/dev/null 2>&1 || true
	- docker volume rm $$(docker volume ls -q | grep -Ei 'rabbit|mq' || true) >/dev/null 2>&1 || true
	docker run -d --name $(RABBIT_NAME) --network $(NET) \
	  -p $(RABBIT_PORT_EXT):5672 -p $(RABBIT_MGMT_PORT_EXT):15672 \
	  -e RABBITMQ_DEFAULT_USER=$(AMQP_USER) \
	  -e RABBITMQ_DEFAULT_PASS=$(AMQP_PASS) \
	  $(RABBIT_IMG)

rabbit-wait:
	@echo "Esperando a RabbitMQ (healthcheck con timeout)…"
	@set -e; ok=0; \
	for i in $$(seq 1 45); do \
	  if docker exec $(RABBIT_NAME) rabbitmq-diagnostics -q check_running >/dev/null 2>&1; then \
	    ok=1; break; \
	  fi; sleep 2; \
	done; \
	if [ $$ok -ne 1 ]; then \
	  echo "RabbitMQ NO arrancó, logs:"; docker logs --tail 200 $(RABBIT_NAME); exit 1; \
	fi; \
	docker port $(RABBIT_NAME)

logs: ; docker logs -f $(RABBIT_NAME)
logs-rabbit: ; docker logs -f $(RABBIT_NAME)
ps: ; docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
stop: ; - docker rm -f $(RABBIT_NAME) >/dev/null 2>&1 || true
clean: stop ; - docker volume rm $$(docker volume ls -q | grep -Ei 'rabbit|mq' || true) >/dev/null 2>&1 || true ; - docker network rm $(NET) >/dev/null 2>&1 || true
