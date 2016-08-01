package proxy

import (
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/yinqiwen/gsnova/common/gfwlist"
	"github.com/yinqiwen/gsnova/local/hosts"
)

var GConf LocalConfig
var mygfwlist *gfwlist.GFWList

func matchHostnames(pattern, host string) bool {
	host = strings.TrimSuffix(host, ".")
	pattern = strings.TrimSuffix(pattern, ".")

	if len(pattern) == 0 || len(host) == 0 {
		return false
	}

	patternParts := strings.Split(pattern, ".")
	hostParts := strings.Split(host, ".")

	if len(patternParts) != len(hostParts) {
		return false
	}

	for i, patternPart := range patternParts {
		if i == 0 && patternPart == "*" {
			continue
		}
		if patternPart != hostParts[i] {
			return false
		}
	}
	return true
}

type PAASConfig struct {
	Enable         bool
	ServerList     []string
	ConnsPerServer int
	SNIProxy       string
}

type GAEConfig struct {
	Enable         bool
	ServerList     []string
	SNI            []string
	InjectRange    []string
	ConnsPerServer int
}

type VPSConfig struct {
	Enable         bool
	Server         string
	ConnsPerServer int
}

type PACConfig struct {
	Method   []string
	Host     []string
	Path     []string
	Rule     []string
	Protocol []string
	Remote   string

	methodRegex []*regexp.Regexp
	hostRegex   []*regexp.Regexp
	pathRegex   []*regexp.Regexp
}

func (pac *PACConfig) ruleInHosts(req *http.Request) bool {
	return hosts.InHosts(req.Host)
}

func (pac *PACConfig) matchProtocol(protocol string) bool {
	if len(pac.Protocol) == 0 {
		return true
	}
	for _, p := range pac.Protocol {
		if p == "*" || strings.EqualFold(p, protocol) {
			return true
		}
	}
	return false
}

func (pac *PACConfig) matchRules(req *http.Request) bool {
	if len(pac.Rule) == 0 {
		return true
	}
	if nil == req {
		return false
	}
	ok := true
	for _, rule := range pac.Rule {
		not := false
		if strings.HasPrefix(rule, "!") {
			not = true
			rule = rule[1:]
		}
		if strings.EqualFold(rule, "InHosts") {
			ok = pac.ruleInHosts(req)
		} else if strings.EqualFold(rule, "BlockedByGFW") {
			if nil != mygfwlist {
				ok = mygfwlist.IsBlockedByGFW(req)
			} else {
				log.Printf("NIL GFWList object")
			}
		}
		if not {
			ok = ok != true
		}
		if !ok {
			break
		}
	}
	return ok
}

func MatchRegexs(str string, rules []*regexp.Regexp) bool {
	if len(rules) == 0 {
		return true
	}
	str = strings.ToLower(str)
	for _, regex := range rules {
		if regex.MatchString(str) {
			return true
		}
	}
	return false
}
func NewRegex(rules []string) ([]*regexp.Regexp, error) {
	regexs := make([]*regexp.Regexp, 0)
	for _, originrule := range rules {
		if originrule == "*" && len(rules) == 1 {
			break
		}
		rule := strings.Replace(strings.ToLower(originrule), "*", ".*", -1)
		reg, err := regexp.Compile(rule)
		if nil != err {
			log.Printf("Invalid pattern:%s for reason:%v", originrule, err)
			return nil, err
		} else {
			regexs = append(regexs, reg)
		}
	}

	return regexs, nil
}

func (pac *PACConfig) Match(protocol string, req *http.Request) bool {
	ret := pac.matchProtocol(protocol)
	if !ret {
		return false
	}
	ret = pac.matchRules(req)
	if !ret {
		return false
	}
	if nil == req {
		if len(pac.hostRegex) > 0 || len(pac.methodRegex) > 0 || len(pac.pathRegex) > 0 {
			return false
		}
		return true
	}
	return MatchRegexs(req.Host, pac.hostRegex) && MatchRegexs(req.Method, pac.methodRegex) && MatchRegexs(req.URL.Path, pac.pathRegex)
}

type ProxyConfig struct {
	Local string
	PAC   []PACConfig
}

func (cfg *ProxyConfig) findProxyByRequest(proto string, req *http.Request) (Proxy, string) {
	var p Proxy
	var proxyName string
	for _, pac := range cfg.PAC {
		if pac.Match(proto, req) {
			p = getProxyByName(pac.Remote)
			proxyName = pac.Remote
			break
		}

	}
	if nil == p {
		log.Printf("No proxy found.")
	}
	return p, proxyName
}

type DirectConfig struct {
	SNI []string
}

type EncryptConfig struct {
	Method string
	Key    string
}

type LocalConfig struct {
	Log       []string
	Encrypt   EncryptConfig
	UserAgent string
	Auth      string
	UDPGWAddr string
	Proxy     []ProxyConfig
	PAAS      PAASConfig
	GAE       GAEConfig
	VPS       VPSConfig
	Direct    DirectConfig
}

func (cfg *LocalConfig) init() error {
	for i, _ := range cfg.Proxy {
		for j, pac := range cfg.Proxy[i].PAC {
			cfg.Proxy[i].PAC[j].methodRegex, _ = NewRegex(pac.Method)
			cfg.Proxy[i].PAC[j].hostRegex, _ = NewRegex(pac.Host)
			cfg.Proxy[i].PAC[j].pathRegex, _ = NewRegex(pac.Path)
		}
	}
	// gfwlist, err := gfwlist.NewGFWList("https://raw.githubusercontent.com/gfwlist/gfwlist/master/gfwlist.txt", "", true)
	// if nil != err {
	// 	return err
	// }
	// mygfwlist = gfwlist
	return nil
}
