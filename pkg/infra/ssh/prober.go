// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	propolisssh "github.com/stacklok/propolis/ssh"

	"github.com/stacklok/waggle/pkg/domain/environment"
)

// Prober inspects a running environment for available capabilities via SSH.
type Prober struct{}

// NewProber creates a new SSH-based Prober.
func NewProber() *Prober {
	return &Prober{}
}

// Probe collects runtime capability details from the guest.
func (*Prober) Probe(
	ctx context.Context, host string, port uint16, keyPath string,
) (environment.Capabilities, error) {
	const trueValue = "true"

	client := propolisssh.NewClient(host, port, "sandbox", keyPath)

	script := strings.TrimSpace(`
python_cmd=""
if command -v python3 >/dev/null 2>&1; then
  python_cmd=$(command -v python3)
elif command -v python >/dev/null 2>&1; then
  python_cmd=$(command -v python)
fi

pip_cmd=""
if command -v pip3 >/dev/null 2>&1; then
  pip_cmd=$(command -v pip3)
elif command -v pip >/dev/null 2>&1; then
  pip_cmd=$(command -v pip)
fi

node_cmd=""
if command -v node >/dev/null 2>&1; then
  node_cmd=$(command -v node)
fi

npm_cmd=""
if command -v npm >/dev/null 2>&1; then
  npm_cmd=$(command -v npm)
fi

venv="false"
if [ -n "$python_cmd" ]; then
  "$python_cmd" -c "import venv" >/dev/null 2>&1 && venv="true"
fi

apk="false"
if command -v apk >/dev/null 2>&1; then
  apk="true"
fi

apt_get="false"
if command -v apt-get >/dev/null 2>&1; then
  apt_get="true"
fi

dnf="false"
if command -v dnf >/dev/null 2>&1; then
  dnf="true"
fi

yum="false"
if command -v yum >/dev/null 2>&1; then
  yum="true"
fi

zypper="false"
if command -v zypper >/dev/null 2>&1; then
  zypper="true"
fi

is_root="false"
if [ "$(id -u 2>/dev/null)" = "0" ]; then
  is_root="true"
fi

echo "PATH=$PATH"
echo "PYTHON=$python_cmd"
echo "PIP=$pip_cmd"
echo "NODE=$node_cmd"
echo "NPM=$npm_cmd"
echo "VENV=$venv"
echo "APK=$apk"
echo "APT_GET=$apt_get"
echo "DNF=$dnf"
echo "YUM=$yum"
echo "ZYPPER=$zypper"
echo "IS_ROOT=$is_root"
`)

	encoded := base64.StdEncoding.EncodeToString([]byte(script))
	cmd := fmt.Sprintf(
		"printf '%%s' %s | base64 -d | sh",
		propolisssh.ShellEscape(encoded),
	)

	output, runErr := client.Run(ctx, cmd)
	if runErr != nil {
		return environment.Capabilities{}, fmt.Errorf("probe environment: %w", runErr)
	}

	values := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		values[parts[0]] = parts[1]
	}

	return environment.Capabilities{
		DetectedAt:    time.Now(),
		Path:          values["PATH"],
		PythonCommand: values["PYTHON"],
		PipCommand:    values["PIP"],
		NodeCommand:   values["NODE"],
		NpmCommand:    values["NPM"],
		HasVenv:       values["VENV"] == trueValue,
		HasApk:        values["APK"] == trueValue,
		HasAptGet:     values["APT_GET"] == trueValue,
		HasDnf:        values["DNF"] == trueValue,
		HasYum:        values["YUM"] == trueValue,
		HasZypper:     values["ZYPPER"] == trueValue,
		IsRoot:        values["IS_ROOT"] == trueValue,
	}, nil
}
