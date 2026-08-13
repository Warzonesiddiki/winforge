//go:build windows

package maintenance

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"winforge/internal/service"
)

// runStream executes a command, streaming each output line to log (if non-nil).
func runStream(log LogFunc, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout // DISM/netsh write progress to stderr on some versions

	if err := cmd.Start(); err != nil {
		return err
	}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		if log != nil {
			log(sc.Text())
		}
	}
	return cmd.Wait()
}

// run runs a command and returns its combined output on failure.
func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, args, err, out)
	}
	return nil
}

func resetWindowsUpdate(log LogFunc) error {
	for _, svc := range []string{"wuauserv", "bits", "cryptSvc"} {
		_ = service.Stop(svc)
	}

	var renameErr error
	dir := filepath.Join(os.Getenv("SystemRoot"), "SoftwareDistribution")
	bak := dir + ".bak"
	_ = os.RemoveAll(bak)
	if _, err := os.Stat(dir); err == nil {
		renameErr = os.Rename(dir, bak)
	} else if !os.IsNotExist(err) {
		renameErr = err
	}

	for _, svc := range []string{"wuauserv", "bits"} {
		_ = service.Start(svc)
	}
	return renameErr
}

func repairImage(log LogFunc) error {
	return runStream(log, "dism.exe", "/Online", "/Cleanup-Image", "/RestoreHealth")
}

func flushDNS() error {
	return run("ipconfig", "/flushdns")
}

func networkReset(log LogFunc) error {
	steps := [][]string{
		{"netsh", "winsock", "reset"},
		{"netsh", "int", "ip", "reset"},
		{"ipconfig", "/release"},
		{"ipconfig", "/renew"},
		{"ipconfig", "/flushdns"},
	}
	var firstErr error
	for _, s := range steps {
		if err := runStream(log, s[0], s[1:]...); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func enableFeature(name string, log LogFunc) error {
	return runStream(log, "dism.exe", "/Online", "/Enable-Feature", "/FeatureName:"+name, "/All", "/NoRestart")
}

func disableFeature(name string, log LogFunc) error {
	return runStream(log, "dism.exe", "/Online", "/Disable-Feature", "/FeatureName:"+name, "/NoRestart")
}

func removeProvisionedAppx(packageName string) error {
	return run("dism.exe", "/Online", "/Remove-ProvisionedAppxPackage", "/PackageName:"+packageName, "/NoRestart")
}

func setDns(adapter, primary, secondary string) error {
	nameArg := fmt.Sprintf(`name="%s"`, adapter)
	if err := run("netsh", "interface", "ip", "set", "dns", nameArg, "static", primary); err != nil {
		return err
	}
	if secondary != "" {
		_ = run("netsh", "interface", "ip", "add", "dns", nameArg, secondary, "index=2")
	}
	return nil
}

func setDnsOnAll(primary, secondary string) error {
	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	var firstErr error
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		if err := setDns(iface.Name, primary, secondary); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
