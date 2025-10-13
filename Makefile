.PHONY: up upild down logs logs2file

up:
	@docker compose up -d

upild:
	@docker compose up -d --build

down:
	@docker compose down --remove-orphans

logs:
	@docker compose logs consumer producer -f

logs2file:
	@docker compose logs consumer > logs/consumer.log
	@docker compose logs producer > logs/producer.log

pause:
	@docker compose pause producer consumer

resume:
	@docker compose unpause producer consumer
