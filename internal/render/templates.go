package render

const FormatURL = "url"

const (
	pgSSL = `(index .params "sslmode")`
	pgEnv = `{{envline "PGHOST" .host}}{{envline "PGPORT" .port}}{{envline "PGUSER" .username}}` +
		`{{envline "PGPASSWORD" .password}}{{envline "PGDATABASE" .database}}{{envline "PGSSLMODE" ` + pgSSL + `}}`
	pgDotenv = `{{dotenv "PGHOST" .host}}{{dotenv "PGPORT" .port}}{{dotenv "PGUSER" .username}}` +
		`{{dotenv "PGPASSWORD" .password}}{{dotenv "PGDATABASE" .database}}{{dotenv "PGSSLMODE" ` + pgSSL + `}}`
	redisCLI = `{{envset "REDISCLI_AUTH" .password}}redis-cli{{with .host}} -h {{sh .}}{{end}}` +
		`{{with .port}} -p {{sh .}}{{end}}{{with .username}} --user {{sh .}}{{end}}` +
		`{{with .database}} -n {{sh .}}{{end}}{{if eq .scheme "rediss"}} --tls{{end}}{{envunset "REDISCLI_AUTH" .password}}`
)

var builtins = map[string]map[string]string{
	Postgres: {
		"url":        `{{.url}}`,
		"env":        pgEnv,
		"dotenv":     pgDotenv,
		"psql":       `psql {{sh .url}}`,
		"pg_dump":    `pg_dump {{sh .url}}`,
		"pg_restore": `pg_restore -d {{sh .url}}`,
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
