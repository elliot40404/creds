package render

const FormatURL = "url"

const (
	pgSSL       = `(index .params "sslmode")`
	pgEnvPrefix = `{{envset "PGPASSWORD" .password}}{{envset "PGSSLMODE" ` + pgSSL + `}}`
	pgEnvSuffix = `{{envunset "PGPASSWORD" .password}}{{envunset "PGSSLMODE" ` + pgSSL + `}}`
	pgFlags     = `{{with .host}} -h {{sh .}}{{end}}{{with .port}} -p {{sh .}}{{end}}{{with .username}} -U {{sh .}}{{end}}{{with .database}} -d {{sh .}}{{end}}`
	pgEnv       = `{{envline "PGHOST" .host}}{{envline "PGPORT" .port}}{{envline "PGUSER" .username}}` +
		`{{envline "PGPASSWORD" .password}}{{envline "PGDATABASE" .database}}{{envline "PGSSLMODE" ` + pgSSL + `}}`
	pgDotenv = `{{dotenv "PGHOST" .host}}{{dotenv "PGPORT" .port}}{{dotenv "PGUSER" .username}}` +
		`{{dotenv "PGPASSWORD" .password}}{{dotenv "PGDATABASE" .database}}{{dotenv "PGSSLMODE" ` + pgSSL + `}}`
	redisCLI = `{{envset "REDISCLI_AUTH" .password}}redis-cli{{with .host}} -h {{sh .}}{{end}}` +
		`{{with .port}} -p {{sh .}}{{end}}{{with .username}} --user {{sh .}}{{end}}` +
		`{{with .database}} -n {{sh .}}{{end}}{{if eq .scheme "rediss"}} --tls{{end}}{{envunset "REDISCLI_AUTH" .password}}`
)

func pgCmd(name string) string {
	return pgEnvPrefix + name + pgFlags + pgEnvSuffix
}

var builtins = map[string]map[string]string{
	Postgres: {
		"url":        `{{.url}}`,
		"env":        pgEnv,
		"dotenv":     pgDotenv,
		"psql":       pgCmd("psql"),
		"pg_dump":    pgCmd("pg_dump"),
		"pg_restore": pgCmd("pg_restore"),
	},
	Redis: {
		"url":       `{{.url}}`,
		"redis-cli": redisCLI,
		"env":       `{{envline "REDIS_URL" .url}}`,
		"dotenv":    `{{dotenv "REDIS_URL" .url}}`,
	},
	Mongo: {
		"url":          `{{.url}}`,
		"mongosh":      `mongosh {{sh .url}}`,
		"mongodump":    `mongodump {{flag "--uri=" .url}}`,
		"mongorestore": `mongorestore {{flag "--uri=" .url}}`,
		"env":          `{{envline "MONGODB_URI" .url}}`,
		"dotenv":       `{{dotenv "MONGODB_URI" .url}}`,
	},
}

var defaultScheme = map[string]string{
	Postgres: "postgresql",
	Redis:    "redis",
	Mongo:    "mongodb",
}
