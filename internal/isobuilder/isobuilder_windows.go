//go:build windows

package isobuilder

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	return cmd.Wait()
}

func runOutput(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func listEditions(sourceDir string) ([]Edition, error) {
	img, err := imageFile(sourceDir)
	if err != nil {
		return nil, err
	}
	out, err := runOutput("dism.exe", "/Get-WimInfo", "/WimFile:"+img)
	if err != nil {
		return nil, err
	}
	return parseWimInfo(out), nil
}

func build(opts Options) (Result, error) {
	res := Result{SourceDir: opts.SourceDir}
	if err := ValidateOptions(&opts); err != nil {
		return res, err
	}

	sourceDir := opts.SourceDir
	var cleanup []string
	defer func() {
		for _, d := range cleanup {
			_ = os.RemoveAll(d)
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
		_ = tmpWim.Close()
		_ = os.Remove(tmpWimName)
		cleanup = append(cleanup, tmpWimName)

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
			if err := runStream(opts.Log, "dism.exe", args...); err != nil {
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
			_ = os.Remove(filepath.Join(workDir, "sources", name))
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

// copyTree copies src into dst, skipping the original image file (it is
// replaced with the slimmed one by the caller).
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		switch rel {
		case filepath.Join("sources", "install.wim"), filepath.Join("sources", "install.esd"):
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			_ = in.Close()
			return err
		}
		_, cerr := io.Copy(out, in)
		_ = in.Close()
		if err := out.Close(); cerr == nil {
			cerr = err
		}
		return cerr
	})
}

func buildISO(sourceDir, isoPath, label string, log LogFunc) error {
	if _, err := exec.LookPath("oscdimg"); err != nil {
		return ErrOscdimgMissing
	}
	bootData, err := bootDataArg(sourceDir)
	if err != nil {
		return err
	}
	args := []string{"-m", "-o", "-u2", "-udfver102", "-bootdata:" + bootData, "-l" + label, sourceDir, isoPath}
	return runStream(log, "oscdimg", args...)
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
