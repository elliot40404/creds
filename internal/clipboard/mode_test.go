package clipboard

import (
	"maps"
	"testing"
)

func TestResolve(t *testing.T) {
	ssh := map[string]string{"SSH_CONNECTION": "10.0.0.1 5000 10.0.0.2 22"}
	tty := map[string]string{"SSH_TTY": "/dev/pts/1"}
	client := map[string]string{"SSH_CLIENT": "10.0.0.1 5000 22"}
	herdr := map[string]string{"HERDR_ENV": "1"}
	herdrSSH := maps.Clone(herdr)
	maps.Copy(herdrSSH, ssh)
	cases := []struct {
		name string
		goos string
		mode Mode
		env  map[string]string
		want Mode
	}{
		{"local", "linux", ModeAuto, nil, ModeNative},
		{"ssh connection", "linux", ModeAuto, ssh, ModeOSC52},
		{"ssh tty", "darwin", ModeAuto, tty, ModeOSC52},
		{"ssh client", "windows", ModeAuto, client, ModeOSC52},
		{"linux herdr pane", "linux", ModeAuto, herdr, ModeOSC52},
		{"mac herdr pane", "darwin", ModeAuto, herdr, ModeOSC52},
		{"windows herdr pane", "windows", ModeAuto, herdr, ModeNative},
		{"windows herdr over ssh", "windows", ModeAuto, herdrSSH, ModeOSC52},
		{"force native in herdr", "linux", ModeNative, herdr, ModeNative},
		{"force native over ssh", "linux", ModeNative, ssh, ModeNative},
		{"force osc52 local", "windows", ModeOSC52, nil, ModeOSC52},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Resolve(c.mode, func(k string) string { return c.env[k] }, c.goos)
			if got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}
