# Laboratorio 1
SOLAMENTE FASE 1 y 2.
## Requisitos
- **Docker Engine** y **Docker Compose** instalados.

### Docker + Docker Compose (Ubuntu)
```bash
sudo apt update
sudo apt install -y docker.io docker-compose-plugin
sudo systemctl enable --now docker
sudo usermod -aG docker $USER && newgrp docker
docker compose version
```
## Instrucciones de ejecución
### 1) Ir a la raíz del proyecto
```
cd sd-lab1
(o la carpeta donde está docker-compose.yml)
```
### 2) Construir las imágenes
```
docker compose build
```
### 3) Levantar los servicios
```
docker compose up
```
## Término (Bajar imágen docker)
```
docker compose down
'''
