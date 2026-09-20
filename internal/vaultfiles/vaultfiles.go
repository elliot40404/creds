package vaultfiles

import (
	"fmt"
	"slices"
	"strings"
)

const (
	EntriesDir   = "entries"
	BucketSuffix = ".enc"
	IgnoreFile   = ".gitignore"
	MetaFile     = "vault.json"
	PasswordFile = "identity.pw.age"
	RecoveryFile = "identity.recovery.age"
	ManifestFile = "manifest.age"
	BucketCount  = 16

	MaxFileSize    = 64 << 20
	DefaultHistory = 3

	IgnoreRules = "*\n" +
		"!" + IgnoreFile + "\n" +
		"!" + MetaFile + "\n" +
		"!" + PasswordFile + "\n" +
		"!" + RecoveryFile + "\n" +
		"!" + ManifestFile + "\n" +
		"!" + EntriesDir + "/\n" +
		"!" + EntriesDir + "/*" + BucketSuffix + "\n"
)

var fixed = []string{IgnoreFile, MetaFile, PasswordFile, RecoveryFile, ManifestFile}

func Fixed() []string {
	return slices.Clone(fixed)
}

func Allowed(path string) bool {
	if slices.Contains(fixed, path) {
		return true
	}
	name, ok := strings.CutPrefix(path, EntriesDir+"/")
	return ok && IsBucket(name)
}

func IsBucket(name string) bool {
	return len(name) == 2+len(BucketSuffix) && name[0] == '0' && isHex(name[1]) && name[2:] == BucketSuffix
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f'
}

func BucketName(slot int) string {
	return fmt.Sprintf("%02x", slot) + BucketSuffix
}

func Required() []string {
	out := Fixed()
	for slot := range BucketCount {
		out = append(out, EntriesDir+"/"+BucketName(slot))
	}
	return out
}

func Covered() []string {
	out := make([]string, 0, len(fixed)+BucketCount-1)
	for _, p := range Required() {
		if p != ManifestFile {
			out = append(out, p)
		}
	}
	return out
}
