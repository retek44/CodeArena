# Atlas configuration for the migration task (image: backend/Dockerfile.migrate).
# The password comes from the RDS-managed secret (ECS injects DB_PASSWORD).
env "deploy" {
  url = "postgres://${getenv("DB_USER")}:${urlescape(getenv("DB_PASSWORD"))}@${getenv("DB_HOST")}:5432/${getenv("DB_NAME")}?sslmode=${getenv("DB_SSLMODE")}"
  migration {
    dir = "file:///migrations"
  }
}
