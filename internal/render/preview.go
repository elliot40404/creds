package render

const sampleWord = "sample-not-a-secret"

var samples = map[string]string{
	Postgres: "postgres://demo:" + sampleWord + "@db.example.com:5432/demo",
	Redis:    "redis://:" + sampleWord + "@cache.example.com:6379/0",
	Mongo:    "mongodb://demo:" + sampleWord + "@db.example.com:27017/demo",
}

func SampleConn(engine string) (Conn, error) {
	return Parse(engine, samples[engine])
}

func Preview(engine, format, src string, shell Shell) (string, error) {
	r, err := New(map[string]string{engine + "." + format: src}, shell)
	if err != nil {
		return "", err
	}
	c, err := SampleConn(engine)
	if err != nil {
		return "", err
	}
	return r.Render(engine, format, c)
}
