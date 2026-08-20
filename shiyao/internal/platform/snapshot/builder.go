package snapshot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Builder struct {
	Registry   *Registry
	Kernel     string
	GuestAgent string
	WorkDir    string
}

func NewBuilder(registry *Registry, kernel, guestAgent string) (*Builder, error) {
	if registry == nil {
		return nil, fmt.Errorf("snapshot registry is required")
	}
	if kernel == "" {
		return nil, fmt.Errorf("kernel path is required")
	}
	if guestAgent == "" {
		return nil, fmt.Errorf("guest agent path is required")
	}
	return &Builder{Registry: registry, Kernel: kernel, GuestAgent: guestAgent}, nil
}

func (b *Builder) Build(cfg Config, rootfs string) (Manifest, error) {
	if err := cfg.Validate(); err != nil {
		return Manifest{}, err
	}
	if rootfs == "" {
		return Manifest{}, fmt.Errorf("rootfs path is required")
	}
	if _, err := os.Stat(b.Kernel); err != nil {
		return Manifest{}, fmt.Errorf("stat kernel: %w", err)
	}
	if _, err := os.Stat(b.GuestAgent); err != nil {
		return Manifest{}, fmt.Errorf("stat guest agent: %w", err)
	}
	if _, err := exec.LookPath("debootstrap"); err != nil {
		return Manifest{}, fmt.Errorf("debootstrap is required: %w", err)
	}
	workDir := b.WorkDir
	if workDir == "" {
		workDir = filepath.Dir(rootfs)
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return Manifest{}, fmt.Errorf("create build directory: %w", err)
	}
	buildDir, err := os.MkdirTemp(workDir, ".shiyao-build-")
	if err != nil {
		return Manifest{}, fmt.Errorf("create build workspace: %w", err)
	}
	defer os.RemoveAll(buildDir)
	rootDir := filepath.Join(buildDir, "rootfs")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return Manifest{}, fmt.Errorf("create rootfs directory: %w", err)
	}
	if err := b.buildRootfs(context.Background(), cfg, rootDir); err != nil {
		return Manifest{}, err
	}
	if err := installGuestAgent(rootDir, b.GuestAgent); err != nil {
		return Manifest{}, fmt.Errorf("install guest agent: %w", err)
	}
	if err := writeEnvironment(rootDir, cfg.Env); err != nil {
		return Manifest{}, fmt.Errorf("write environment: %w", err)
	}
	if err := createFilesystemImage(rootDir, rootfs, cfg.Resources.DiskMB); err != nil {
		return Manifest{}, err
	}
	manifest := Manifest{Version: ManifestVersion, Name: cfg.Name, ConfigDigest: configDigest(cfg), KernelPath: filepath.Clean(b.Kernel), RootfsPath: filepath.Clean(rootfs), CreatedAt: time.Now().UTC()}
	if err := b.Registry.Put(manifest); err != nil {
		return Manifest{}, fmt.Errorf("register snapshot: %w", err)
	}
	return manifest, nil
}

func (b *Builder) buildRootfs(ctx context.Context, cfg Config, rootDir string) error {
	release, mirror, err := distroSource(cfg.Runtime.Distro)
	if err != nil {
		return err
	}
	if err := runCommand(ctx, "debootstrap", "--variant=minbase", release, rootDir, mirror); err != nil {
		return fmt.Errorf("bootstrap %s: %w", cfg.Runtime.Distro, err)
	}
	if data, err := os.ReadFile("/etc/resolv.conf"); err == nil {
		_ = os.WriteFile(filepath.Join(rootDir, "etc", "resolv.conf"), data, 0o644)
	}
	if err := withChrootMounts(ctx, rootDir, func() error {
		if err := runChroot(ctx, rootDir, "apt-get", "update"); err != nil {
			return fmt.Errorf("apt update: %w", err)
		}
		if len(cfg.Dependencies.System) > 0 {
			args := append([]string{"apt-get", "install", "-y", "--no-install-recommends"}, cfg.Dependencies.System...)
			if err := runChroot(ctx, rootDir, args...); err != nil {
				return fmt.Errorf("install system dependencies: %w", err)
			}
		}
		if err := installLanguage(ctx, rootDir, cfg.Language); err != nil {
			return err
		}
		if len(cfg.Dependencies.Pip) > 0 {
			args := append([]string{"python3", "-m", "pip", "install"}, cfg.Dependencies.Pip...)
			if err := runChroot(ctx, rootDir, args...); err != nil {
				return fmt.Errorf("install pip dependencies: %w", err)
			}
		}
		if len(cfg.Dependencies.NPM) > 0 {
			args := append([]string{"npm", "install", "-g"}, cfg.Dependencies.NPM...)
			if err := runChroot(ctx, rootDir, args...); err != nil {
				return fmt.Errorf("install npm dependencies: %w", err)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func installGuestAgent(rootDir, guestAgent string) error {
	dst := filepath.Join(rootDir, "usr", "local", "bin", "shiyao-agent")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	src, err := os.Open(guestAgent)
	if err != nil {
		return err
	}
	defer src.Close()
	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, src); err != nil {
		return err
	}
	return dstFile.Chmod(0o755)
}

func installLanguage(ctx context.Context, rootDir string, language LanguageConfig) error {
	if language.Name == "" {
		return nil
	}
	packages := map[string][]string{"python": {"python3", "python3-pip"}, "node": {"nodejs", "npm"}, "go": {"golang"}}
	pkgs, ok := packages[strings.ToLower(language.Name)]
	if !ok {
		return fmt.Errorf("unsupported language runtime %q", language.Name)
	}
	args := append([]string{"apt-get", "install", "-y", "--no-install-recommends"}, pkgs...)
	if err := runChroot(ctx, rootDir, args...); err != nil {
		return fmt.Errorf("install %s runtime: %w", language.Name, err)
	}
	return nil
}

func withChrootMounts(ctx context.Context, rootDir string, fn func() error) error {
	dev, proc := filepath.Join(rootDir, "dev"), filepath.Join(rootDir, "proc")
	if err := runCommand(ctx, "mount", "--rbind", "/dev", dev); err != nil {
		return fmt.Errorf("mount /dev: %w", err)
	}
	defer runCommand(context.Background(), "umount", "-R", dev)
	if err := runCommand(ctx, "mount", "-t", "proc", "proc", proc); err != nil {
		return fmt.Errorf("mount /proc: %w", err)
	}
	defer runCommand(context.Background(), "umount", proc)
	return fn()
}

func runChroot(ctx context.Context, rootDir string, args ...string) error {
	return runCommand(ctx, "chroot", append([]string{rootDir}, args...)...)
}
func runCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}
func writeEnvironment(rootDir string, env map[string]string) error {
	if len(env) == 0 {
		return nil
	}
	path := filepath.Join(rootDir, "etc", "shiyao-env")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var lines []string
	for key, value := range env {
		if !validEnvName(key) {
			return fmt.Errorf("invalid environment variable name %q", key)
		}
		lines = append(lines, fmt.Sprintf("%s=%s", key, shellQuote(value)))
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
func createFilesystemImage(rootDir, output string, diskMB int) error {
	if _, err := exec.LookPath("mkfs.ext4"); err != nil {
		return fmt.Errorf("mkfs.ext4 is required: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("create image directory: %w", err)
	}
	if err := runCommand(context.Background(), "dd", "if=/dev/zero", "of="+output, "bs=1M", "count="+strconv.Itoa(diskMB), "status=progress"); err != nil {
		return fmt.Errorf("allocate filesystem image: %w", err)
	}
	if err := runCommand(context.Background(), "mkfs.ext4", "-F", output); err != nil {
		return fmt.Errorf("format filesystem image: %w", err)
	}
	mountDir, err := os.MkdirTemp(filepath.Dir(output), ".shiyao-mount-")
	if err != nil {
		return fmt.Errorf("create image mount directory: %w", err)
	}
	defer os.RemoveAll(mountDir)
	if err := runCommand(context.Background(), "mount", "-o", "loop", output, mountDir); err != nil {
		return fmt.Errorf("mount filesystem image: %w", err)
	}
	defer runCommand(context.Background(), "umount", mountDir)
	cmd := exec.Command("sh", "-c", "tar --exclude=./dev -C \"$1\" -cpf - . | tar -C \"$2\" -xpf -", "sh", rootDir, mountDir)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("populate filesystem image: %w", err)
	}
	return nil
}
func configDigest(cfg Config) string {
	payload := fmt.Sprintf("%#v", cfg)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}
func distroSource(distro string) (string, string, error) {
	switch {
	case strings.HasPrefix(distro, "ubuntu-"):
		return strings.TrimPrefix(distro, "ubuntu-"), "http://archive.ubuntu.com/ubuntu", nil
	case strings.HasPrefix(distro, "debian-"):
		return strings.TrimPrefix(distro, "debian-"), "http://deb.debian.org/debian", nil
	default:
		return "", "", fmt.Errorf("unsupported distribution %q", distro)
	}
}
func validEnvName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
