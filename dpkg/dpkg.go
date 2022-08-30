package dpkg

import (
	"encoding/json"
	"os/exec"
)

type Version struct {
	Version string
	Status  string
}

func Query(packageName string) (*Version, error) {
	dpkg := Version{}
	exe := exec.Command("/usr/bin/dpkg-query", "--showformat={\"version\":\"${Version}\",\"status\":\"${Status}\"}", "--show", packageName)
	out, err := exe.CombinedOutput()
	if err != nil {
		return &dpkg, err
	}

	if err := json.Unmarshal(out, &dpkg); err != nil {
		return &dpkg, err
	}

	return &dpkg, nil
}
