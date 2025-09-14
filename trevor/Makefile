COMPOSE = docker-compose -f docker-compose.trevor.yml

up:
	$(COMPOSE) up -d --build

logs:
	$(COMPOSE) logs -f trevor

down:
	$(COMPOSE) down

ps:
	$(COMPOSE) ps

reup: down up
