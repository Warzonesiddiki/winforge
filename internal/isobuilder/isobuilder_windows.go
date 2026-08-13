//go:build windows

package isobuilder

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"winforge/internal/platform"
	"winforge/internal/procout"
	"winforge/internal/winapi"
)

func runStream(log LogFunc, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout // dism/oscdimg write progress to stderr on some versions

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

func runOutput(name string, args ...string) (string, error) {
	out, err := procout.CombinedOutput(exec.Command(name, args...), 4<<20)
	if err != nil {
		return string(out), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func listEditions(sourceDir string) ([]Edition, error) {
	if platform.IsElevated() {
		return nil, errors.New("ISO inspection is disabled while WinForge is elevated; restart normally to inspect user-selected installation media")
	}
	img, err := imageFile(sourceDir)
	if err != nil {
		return nil, err
	}
	out, err := runOutput(winapi.SystemPath("dism.exe"), "/English", "/Get-WimInfo", "/WimFile:"+img)
	if err != nil {
		return nil, err
	}
	editions := parseWimInfo(out)
	if len(editions) == 0 {
		return nil, errors.New("DISM returned no parseable editions")
	}
	return editions, nil
}

func build(opts Options) (res Result, retErr error) {
	res.SourceDir = opts.SourceDir
	if platform.IsElevated() {
		return res, errors.New("ISO building is disabled while WinForge is elevated; restart normally so PATH-discovered oscdimg cannot inherit an administrator token")
	}
	if err := ValidateOptions(&opts); err != nil {
		return res, err
	}

	sourceDir := opts.SourceDir
	var cleanup []string
	defer func() {
		for i := len(cleanup) - 1; i >= 0; i-- {
			if err := os.RemoveAll(cleanup[i]); err != nil {
				retErr = errors.Join(retErr, fmt.Errorf("remove temporary path %q: %w", cleanup[i], err))
			}
		}
	}()

	if len(opts.Editions) > 0 {
		editions, err := listEditions(sourceDir)
		if err != nil {
			return res, err
		}
		indexes, err := selectIndexes(editions, opts.Editions)
		if err != nil {
			return res, err
		}
		nameByIndex := make(map[int]string, len(editions))
		for _, e := range editions {
			nameByIndex[e.Index] = e.Name
		}

		// Export the chosen editions into a fresh, slimmed install.wim.
		tmpWim, err := os.CreateTemp("", "winforge-install-*.wim")
		if err != nil {
			return res, err
		}
		tmpWimName := tmpWim.Name()
		cleanup = append(cleanup, tmpWimName)
		if err := tmpWim.Close(); err != nil {
			return res, fmt.Errorf("close temporary WIM: %w", err)
		}
		if err := os.Remove(tmpWimName); err != nil {
			return res, fmt.Errorf("prepare temporary WIM destination: %w", err)
		}

		img, err := imageFile(sourceDir)
		if err != nil {
			return res, err
		}
		for i, idx := range indexes {
			if opts.Log != nil {
				opts.Log(fmt.Sprintf("Exporting edition %q (index %d)…", nameByIndex[idx], idx))
			}
			args := []string{
				"/Export-Image",
				"/SourceImageFile:" + img,
				fmt.Sprintf("/SourceIndex:%d", idx),
				"/DestinationImageFile:" + tmpWimName,
				"/Compress:max",
			}
			if i == 0 {
				args = append(args, "/CheckIntegrity")
			}
			if err := runStream(opts.Log, winapi.SystemPath("dism.exe"), args...); err != nil {
				return res, err
			}
			res.Exported = append(res.Exported, Edition{Index: idx, Name: nameByIndex[idx]})
		}

		// Copy the source into a scratch dir and swap in the slimmed image so
		// the user's extracted source is never modified.
		workDir, err := os.MkdirTemp("", "winforge-iso-*")
		if err != nil {
			return res, err
		}
		cleanup = append(cleanup, workDir)
		if opts.Log != nil {
			opts.Log("Copying installation source…")
		}
		if err := copyTree(sourceDir, workDir); err != nil {
			return res, err
		}
		for _, name := range []string{"install.wim", "install.esd"} {
			path := filepath.Join(workDir, "sources", name)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return res, fmt.Errorf("remove original image %q: %w", path, err)
			}
		}
		if err := os.Rename(tmpWimName, filepath.Join(workDir, "sources", "install.wim")); err != nil {
			return res, err
		}
		sourceDir = workDir
	}

	if opts.Log != nil {
		opts.Log("Building ISO with oscdimg…")
	}
	if err := buildISO(sourceDir, opts.OutputISO, opts.Label, opts.Log); err != nil {
		return res, err
	}
	res.ISO = opts.OutputISO
	return res, nil
}

func buildISO(sourceDir, isoPath, label string, log LogFunc) error {
	if platform.IsElevated() {
		return errors.New("ISO building is disabled while WinForge is elevated")
	}
	oscdimg, err := exec.LookPath("oscdimg")
	if err != nil {
		return ErrOscdimgMissing
	}
	oscdimg, err = filepath.Abs(oscdimg)
	if err != nil {
		return fmt.Errorf("resolve oscdimg path: %w", err)
	}
	bootData, err := bootDataArg(sourceDir)
	if err != nil {
		return err
	}
	args := []string{"-m", "-o", "-u2", "-udfver102", "-bootdata:" + bootData, "-l" + label, sourceDir, isoPath}
	return runStream(log, oscdimg, args...)
}

// bootDataArg builds the oscdimg -bootdata string from the boot images present
// in the source, supporting BIOS, UEFI, or both.
func bootDataArg(sourceDir string) (string, error) {
	etfs := filepath.Join(sourceDir, "boot", "etfsboot.com")
	efisys := filepath.Join(sourceDir, "efi", "microsoft", "boot", "efisys.bin")
	hasEtfs := fileExists(etfs)
	hasEfisys := fileExists(efisys)

	switch {
	case hasEtfs && hasEfisys:
		return fmt.Sprintf("2#p0,e,b%s#pEF,e,b%s", etfs, efisys), nil
	case hasEtfs:
		return fmt.Sprintf("1#p0,e,b%s", etfs), nil
	case hasEfisys:
		return fmt.Sprintf("1#pEF,e,b%s", efisys), nil
	default:
		return "", fmt.Errorf("boot images not found (expected %q and/or %q)", etfs, efisys)
	}
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
