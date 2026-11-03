package steps

import (
	"errors"
	"fmt"
	"path"
	"strings"
)

// BtrfsOpts son las opciones de montaje de la raíz btrfs.
const BtrfsOpts = "discard=async,space_cache=v2"

// Cmd es un comando externo que un paso pide ejecutar. Los pasos solo
// construyen comandos; quien los ejecuta (el ejecutor real o el dry-run) es
// otra pieza, así los pasos se prueban sin privilegios ni discos.
type Cmd struct {
	Args []string
}

// String devuelve el comando como se escribiría en una shell.
func (c Cmd) String() string {
	q := make([]string, len(c.Args))
	for i, a := range c.Args {
		q[i] = quote(a)
	}
	return strings.Join(q, " ")
}

func cmd(args ...string) Cmd { return Cmd{Args: args} }

func quote(s string) string {
	if s == "" {
		return "''"
	}
	for _, r := range s {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			strings.ContainsRune("_@%+=:,./-", r)
		if !ok {
			return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
		}
	}
	return s
}

// PartitionCmds borra el disco y crea la tabla GPT con las particiones de Layout.
func PartitionCmds(disk string, parts []Part) []Cmd {
	cmds := []Cmd{
		cmd("wipefs", "--all", "--force", disk),
		cmd("sgdisk", "--zap-all", disk),
	}
	for _, p := range parts {
		end := "0"
		if p.SizeMiB > 0 {
			end = fmt.Sprintf("+%dM", p.SizeMiB)
		}
		n := fmt.Sprint(p.Num)
		cmds = append(cmds, cmd("sgdisk", "-n", n+":0:"+end, "-t", n+":"+p.Type, "-c", n+":"+p.Name, disk))
	}
	return append(cmds, cmd("partprobe", disk), cmd("udevadm", "settle"))
}

// FormatCmds crea los sistemas de archivos y el swap de cada partición.
func FormatCmds(parts []Part) []Cmd {
	var cmds []Cmd
	for _, p := range parts {
		switch p.FS {
		case "fat32":
			cmds = append(cmds, cmd("mkfs.fat", "-F", "32", "-n", p.Label, p.Dev))
		case "swap":
			cmds = append(cmds, cmd("mkswap", "-L", p.Label, p.Dev))
		case "btrfs":
			cmds = append(cmds, cmd("mkfs.btrfs", "-f", "-L", p.Label, p.Dev))
		}
	}
	return cmds
}

// MountCmds crea los subvolúmenes btrfs y monta todo bajo target.
func MountCmds(parts []Part, target string) ([]Cmd, error) {
	var root, esp, swap *Part
	for i := range parts {
		p := &parts[i]
		switch {
		case p.Mount == "/":
			root = p
		case p.Mount == "/boot/efi":
			esp = p
		case p.FS == "swap":
			swap = p
		}
	}
	if root == nil {
		return nil, errors.New("no hay partición raíz")
	}
	if esp == nil {
		return nil, errors.New("no hay partición ESP")
	}

	subs := Subvolumes()
	mountSub := func(s Subvol) Cmd {
		return cmd("mount", "-o", "subvol="+s.Name+","+BtrfsOpts, root.Dev, path.Join(target, s.Mount))
	}

	cmds := []Cmd{cmd("mkdir", "-p", target), cmd("mount", root.Dev, target)}
	for _, s := range subs {
		cmds = append(cmds, cmd("btrfs", "subvolume", "create", target+"/"+s.Name))
	}
	cmds = append(cmds, cmd("umount", target), mountSub(subs[0]))

	mk := []string{"mkdir", "-p"}
	for _, s := range subs[1:] {
		mk = append(mk, path.Join(target, s.Mount))
	}
	mk = append(mk, path.Join(target, esp.Mount))
	cmds = append(cmds, cmd(mk...))

	for _, s := range subs[1:] {
		cmds = append(cmds, mountSub(s))
	}
	cmds = append(cmds, cmd("mount", esp.Dev, path.Join(target, esp.Mount)))
	if swap != nil {
		cmds = append(cmds, cmd("swapon", swap.Dev))
	}
	return cmds, nil
}
