package main

type orphanReport struct {
	Filters int
	Routes  []string
	DNS     []string
	Actions []string
}

func reconcileOrphans(filters int, routeLines, dnsLines []string) orphanReport {
	rep := orphanReport{Filters: filters}
	if filters > 0 {
		rep.Actions = append(rep.Actions, "remove-engaged-filters")
	}
	for _, line := range routeLines {
		if routeIsSplit(line) {
			rep.Routes = append(rep.Routes, line)
			rep.Actions = append(rep.Actions, "delete-route "+line)
		}
	}
	for _, line := range dnsLines {
		if dnsIsStale(line) {
			rep.DNS = append(rep.DNS, line)
			rep.Actions = append(rep.Actions, "clear-dns "+line)
		}
	}
	return rep
}

func routeIsSplit(line string) bool {
	return containsToken(line, "0.0.0.0/1") || containsToken(line, "128.0.0.0/1") || containsToken(line, "::/1") || containsToken(line, "8000::/1")
}

func dnsIsStale(line string) bool {
	return containsToken(line, "10.7.0.1")
}

func containsToken(line, token string) bool {
	return len(line) >= len(token) && (line == token || indexOf(line, token) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
