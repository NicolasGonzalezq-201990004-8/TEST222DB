NET                 ?= heistnet
RABBIT_IMG          ?= rabbitmq:3.13-management
RABBIT_NAME         ?= rabbitmq
AMQP_USER           ?= heist
AMQP_PASS           ?= heist123

# crea la red si no existe (no falla si ya existe)
network:
	@docker network inspect $(NET) >/dev/null 2>&1 || docker network create $(NET)

# elimina contenedor/volúmenes anteriores de rabbit
rabbit-clean:
	- docker rm -f $(RABBIT_NAME) >/dev/null 2>&1 || true
	- docker volume rm $$(docker volume ls -q | grep -Ei 'rabbitmq|rabbit' || true) >/dev/null 2>&1 || true

# levanta RabbitMQ publicando 5673->5672 y 15673->15672, con usuario y healthcheck
rabbit: network rabbit-clean
	docker run -d --name $(RABBIT_NAME) --network $(NET) \
		-p 5673:5672 -p 15673:15672 \
		-e RABBITMQ_DEFAULT_USER=$(AMQP_USER) \
		-e RABBITMQ_DEFAULT_PASS=$(AMQP_PASS) \
		--health-cmd="rabbitmq-diagnostics -q check_running" \
		--health-interval=5s --health-timeout=5s --health-retries=60 \
		$(RABBIT_IMG)

rabbit-wait:
	@echo "Esperando a RabbitMQ (healthcheck)…"
	@until [ "$$(docker inspect -f '{{.State.Health.Status}}' $(RABBIT_NAME) 2>/dev/null)" = "healthy" ]; do \
		echo "..."; sleep 2; \
	done
	@echo "RabbitMQ está healthy."

logs-rabbit:
	docker logs -f $(RABBIT_NAME)

ps:
	docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
