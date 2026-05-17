package strace

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
)

type NetCall struct {
	Addr string `json:"addr"`
	Port int    `json:"port"`
}

type ExecCall struct {
	Cmd  string   `json:"cmd"`
	Args []string `json:"args"`
}

type Calls struct {
	Network      []NetCall  `json:"network"`
	Subprocesses []ExecCall `json:"subprocesses"`
}

var (
	inetRe  = regexp.MustCompile(`sin_port=htons\((\d+)\).*?sin_addr=inet_addr\("([^"]+)"\)`)
	inet6Re = regexp.MustCompile(`sin6_port=htons\((\d+)\).*?inet_pton\(AF_INET6,\s*"([^"]+)"`)

	// execve("/path", ["arg0", "arg1", ...], ...)
	execveRe = regexp.MustCompile(`execve\("([^"]+)",\s*\[([^\]]*)\]`)
	argRe    = regexp.MustCompile(`"([^"]*)"`)
)

func Parse(r io.Reader) Calls {
	var (
		calls    Calls
		seenNet  = map[string]bool{}
		seenExec = map[string]bool{}
	)

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()

		if m := inetRe.FindStringSubmatch(line); m != nil {
			port, _ := strconv.Atoi(m[1])
			if port > 0 {
				key := m[2] + ":" + m[1]
				if !seenNet[key] {
					seenNet[key] = true
					calls.Network = append(calls.Network, NetCall{Addr: m[2], Port: port})
				}
			}
		} else if m := inet6Re.FindStringSubmatch(line); m != nil {
			port, _ := strconv.Atoi(m[1])
			if port > 0 {
				key := m[2] + ":" + m[1]
				if !seenNet[key] {
					seenNet[key] = true
					calls.Network = append(calls.Network, NetCall{Addr: m[2], Port: port})
				}
			}
		}

		if m := execveRe.FindStringSubmatch(line); m != nil {
			cmd := m[1]
			if !seenExec[cmd] {
				seenExec[cmd] = true
				var args []string
				for _, am := range argRe.FindAllStringSubmatch(m[2], -1) {
					args = append(args, am[1])
				}
				calls.Subprocesses = append(calls.Subprocesses, ExecCall{Cmd: cmd, Args: args})
			}
		}
	}

	if calls.Network == nil {
		calls.Network = []NetCall{}
	}
	if calls.Subprocesses == nil {
		calls.Subprocesses = []ExecCall{}
	}
	sort.Slice(calls.Network, func(i, j int) bool {
		ki := fmt.Sprintf("%s:%d", calls.Network[i].Addr, calls.Network[i].Port)
		kj := fmt.Sprintf("%s:%d", calls.Network[j].Addr, calls.Network[j].Port)
		return ki < kj
	})
	sort.Slice(calls.Subprocesses, func(i, j int) bool {
		return calls.Subprocesses[i].Cmd < calls.Subprocesses[j].Cmd
	})
	return calls
}
