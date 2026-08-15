//go:build windows

package maintenance

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"winforge/internal/procout"
	"winforge/internal/service"
	"winforge/internal/winapi"
)

func maintenanceCommand(name string) (string, error) {
	switch strings.ToLower(strings.TrimSuffix(name, ".exe")) {
	case "dism":
		return winapi.SystemPath("dism.exe"), nil
	case "ipconfig":
		return winapi.SystemPath("ipconfig.exe"), nil
	case "netsh":
		return winapi.SystemPath("netsh.exe"), nil
	default:
		return "", fmt.Errorf("unsupported maintenance command %q", name)
	}
}

// runStream executes a command, streaming each output line to log (if non-nil).
func runStream(log LogFunc, name string, args ...string) error {
	command, err := maintenanceCommand(name)
	if err != nil {
		return err
	}
	cmd := exec.Command(command, args...)
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
	scanErr := sc.Err()
	var drainErr error
	if scanErr != nil {
		_, drainErr = io.Copy(io.Discard, stdout)
	}
	waitErr := cmd.Wait()
	return errors.Join(scanErr, drainErr, waitErr)
}

// run runs a command and returns its combined output on failure.
func run(name string, args ...string) error {
	command, err := maintenanceCommand(name)
	if err != nil {
		return err
	}
	out, err := procout.CombinedOutput(exec.Command(command, args...), 1<<20)
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, out)
	}
	return nil
}

func resetWindowsUpdate(log LogFunc) error {
	var errs []error
	stopFailed := false
	for _, svc := range []string{"wuauserv", "bits", "cryptSvc"} {
		if err := service.Stop(svc); err != nil {
			errs = append(errs, fmt.Errorf("stop %s: %w", svc, err))
			stopFailed = true
		}
	}

	// Never move the update cache while any dependent service may still be
	// using it. StartService is idempotent, so use the same recovery path for
	// both stop failures and the normal reset completion.
	restartServices := func() {
		for _, svc := range []string{"cryptSvc", "bits", "wuauserv"} {
			if err := service.Start(svc); err != nil {
				errs = append(errs, fmt.Errorf("start %s: %w", svc, err))
			}
		}
	}
	if stopFailed {
		restartServices()
		return errors.Join(errs...)
	}

	// Do not trust the caller-controlled SystemRoot environment variable in an
	// elevated process. Derive the Windows directory from the path returned by
	// GetSystemDirectoryW instead.
	systemRoot := filepath.Dir(winapi.SystemDirectory())
	dir := filepath.Join(systemRoot, "SoftwareDistribution")
	bak := dir + ".bak"
	if err := os.RemoveAll(bak); err != nil {
		errs = append(errs, fmt.Errorf("remove old SoftwareDistribution backup: %w", err))
	} else if _, err := os.Stat(dir); err == nil {
		if err := os.Rename(dir, bak); err != nil {
			errs = append(errs, fmt.Errorf("rename SoftwareDistribution: %w", err))
		}
	} else if !os.IsNotExist(err) {
		errs = append(errs, fmt.Errorf("inspect SoftwareDistribution: %w", err))
	}

	restartServices()
	return errors.Join(errs...)
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
	var errs []error
	for _, s := range steps {
		if err := runStream(log, s[0], s[1:]...); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", strings.Join(s, " "), err))
		}
	}
	return errors.Join(errs...)
}

func enableFeature(name string, log LogFunc) error {
	return runStream(log, "dism.exe", "/Online", "/Enable-Feature", "/FeatureName:"+name, "/All", "/NoRestart")
}

func disableFeature(name string, log LogFunc) error {
	return runStream(log, "dism.exe", "/Online", "/Disable-Feature", "/FeatureName:"+name, "/NoRestart")
}

const (
	provisionedAppxCacheTTL         = 5 * time.Minute
	provisionedAppxMissRefreshAfter = 30 * time.Second
)

var provisionedAppxCache struct {
	sync.Mutex
	loadedAt time.Time
	packages []provisionedPackage
}

func provisionedAppxCacheFresh(maxAge time.Duration) bool {
	if provisionedAppxCache.loadedAt.IsZero() {
		return false
	}
	age := time.Since(provisionedAppxCache.loadedAt)
	return age >= 0 && age < maxAge
}

// refreshProvisionedAppxCache must be called with provisionedAppxCache locked.
func refreshProvisionedAppxCache() error {
	out, err := procout.CombinedOutput(
		exec.Command(winapi.SystemPath("dism.exe"), "/Online", "/Get-ProvisionedAppxPackages", "/English"),
		4<<20,
	)
	if err != nil {
		return fmt.Errorf("list provisioned Appx packages: %w: %s", err, out)
	}
	packages, err := parseProvisionedPackages(string(out))
	if err != nil {
		return err
	}
	provisionedAppxCache.packages = packages
	provisionedAppxCache.loadedAt = time.Now()
	return nil
}

func provisionedAppxCacheContains(name string) bool {
	for _, pkg := range provisionedAppxCache.packages {
		if strings.EqualFold(name, pkg.displayName) || strings.EqualFold(name, pkg.packageName) {
			return true
		}
	}
	return false
}

func removeProvisionedAppx(name string) error {
	provisionedAppxCache.Lock()
	defer provisionedAppxCache.Unlock()

	if !provisionedAppxCacheFresh(provisionedAppxCacheTTL) {
		if err := refreshProvisionedAppxCache(); err != nil {
			return err
		}
	}
	// A positive cache hit is stable enough for the normal five-minute TTL.
	// Refresh older negative hits sooner so packages provisioned externally do
	// not remain invisible for the full TTL. The shared short window prevents a
	// debloat batch from invoking DISM once for every intentionally absent ID.
	if !provisionedAppxCacheContains(name) && !provisionedAppxCacheFresh(provisionedAppxMissRefreshAfter) {
		if err := refreshProvisionedAppxCache(); err != nil {
			return err
		}
	}

	var errs []error
	kept := provisionedAppxCache.packages[:0]
	for _, pkg := range provisionedAppxCache.packages {
		if !strings.EqualFold(name, pkg.displayName) && !strings.EqualFold(name, pkg.packageName) {
			kept = append(kept, pkg)
			continue
		}
		if err := run("dism.exe", "/Online", "/Remove-ProvisionedAppxPackage", "/PackageName:"+pkg.packageName, "/NoRestart", "/English"); err != nil {
			errs = append(errs, fmt.Errorf("remove provisioned package %q: %w", pkg.packageName, err))
			kept = append(kept, pkg) // Retain failed entries so a later call may retry.
		}
	}
	provisionedAppxCache.packages = kept
	return errors.Join(errs...)
}

func setDns(adapter, primary, secondary string) error {
	nameArg := fmt.Sprintf(`name="%s"`, adapter)
	if err := run("netsh", "interface", "ip", "set", "dns", nameArg, "static", primary); err != nil {
		return err
	}
	if secondary != "" {
		if err := run("netsh", "interface", "ip", "add", "dns", nameArg, secondary, "index=2"); err != nil {
			return err
		}
	}
	return nil
}

func setDnsOnAll(primary, secondary string) error {
	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	var errs []error
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		if err := setDns(iface.Name, primary, secondary); err != nil {
			errs = append(errs, fmt.Errorf("adapter %q: %w", iface.Name, err))
		}
	}
	return errors.Join(errs...)
}
