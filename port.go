package gojack

// #include <jack/jack.h>
import "C"
import (
	"fmt"
)

type Port struct {
	p *C.jack_port_t
}

func newPort(p *C.jack_port_t) *Port {
	return &Port{
		p: p,
	}
}

func newPortByName(c *C.jack_client_t, name *C.char) (*Port, error) {
	p := C.jack_port_by_name(c, name)
	if nil == p {
		return nil, fmt.Errorf("no such port: `%s`", C.GoString(name))
	}
	return newPort(p), nil
}

func (p *Port) Name() (string, error) {
	cStr, err := p.name()
	if nil != err {
		return "", err
	}
	out := C.GoString(cStr)
	return out, nil
}

func (p *Port) name() (*C.char, error) {
	n := C.jack_port_name(p.p)
	if nil == n {
		return nil, fmt.Errorf("could not get port name")
	}
	return n, nil
}
