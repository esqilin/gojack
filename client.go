package gojack

// #cgo pkg-config: jack
// #include <stdlib.h>
// #include <string.h>
// #include <jack/jack.h>
// #include <sys/errno.h>
/*
// go cannot call variadic c functions, so we need a wrapper
jack_client_t *jack_client_open_(const char *client_name, jack_options_t options, jack_status_t *status, char *server_name) {
    return jack_client_open(client_name, options, status, server_name);
}

int jackCProcessFun(jack_nframes_t nframes, void *args) {
    return jackGoProcessFun(nframes, args);
}

int jackCShutdownFun(void *args) {
    jackGoProcessFun(args);
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

var JACK_DEFAULT_AUDIO_TYPE *C.char = C.CString(C.JACK_DEFAULT_AUDIO_TYPE)

type Client struct {
	name       *C.char
	server     *C.char
	status     C.jack_status_t
	options    C.jack_options_t
	jackClient *C.jack_client_t
	cStrings   []*C.char
	inPorts    []*Port
	outPorts   []*Port
}

func NewClient(name string) (*Client, error) {
	n := int(C.jack_client_name_size())
	if len(name) > n {
		return nil, fmt.Errorf("JACK client name cannot exceed %d characters", n)
	}

	return &Client{
		name:     C.CString(name),
		cStrings: []*C.char{},
	}, nil
}

func (c *Client) SetOptionSessionId() {
	c.options |= C.JackSessionID
}
func (c *Client) SetOptionNoStartServer() {
	c.options |= C.JackNoStartServer
}
func (c *Client) SetOptionUseExactName() {
	c.options |= C.JackUseExactName
}

func (c *Client) Name() string {
	return C.GoString(c.name)
}

func (c *Client) ServerName(name string) {
	if len(name) <= 0 {
		c.options &^= C.JackServerName
		return
	}
	C.free(unsafe.Pointer(c.server))
	c.server = C.CString(name)
	c.options |= C.JackServerName
}

func (c *Client) IsServerStarted() bool {
	return 0 != c.status&C.JackServerStarted
}

func (c *Client) IsNameNotUnique() bool {
	return 0 != c.status&C.JackNameNotUnique
}

// Open returns the initial(!) JACK buffer size and a possible error
func (c *Client) Open() (int, error) {
	c.jackClient = C.jack_client_open_(c.name, c.options, &c.status, c.server)
	if nil == c.jackClient {
		msg := fmt.Sprintf("cannot open JACK client. error status: 0x%2.0x", c.status)
		if 0 != c.status&C.JackServerFailed {
			msg += ". unable to connect to JACK server"
		}
		return 0, fmt.Errorf(msg)
	}
	if c.IsNameNotUnique() {
		C.free(unsafe.Pointer(c.name))
		cstr := C.jack_get_client_name(c.jackClient) // cstr in C memory, cannot free
		c.name = C.CString(C.GoString(cstr))         // trick: can still free c.name in Close()
	}
	n := int(C.jack_get_buffer_size(c.jackClient))
	return n, nil
}

func (c *Client) Close() {
	C.free(unsafe.Pointer(c.name))
	C.free(unsafe.Pointer(c.server))
	for _, cStr := range c.cStrings {
		C.free(unsafe.Pointer(cStr))
	}
	C.jack_client_close(c.jackClient)
}

func (c *Client) OnProcess(callback processCallback, args interface{}) {
	jpc := &jackProcessCall{
		c:    c,
		fp:   &callback,
		args: &args,
	}
	C.jack_set_process_callback(
		c.jackClient,
		(*[0]byte)(C.jackCProcessFun),
		unsafe.Pointer(jpc),
	)
}

func (c *Client) OnShutdown(callback shutdownCallback, args interface{}) {
	jsc := &jackShutdownCall{
		fp:   &callback,
		args: &args,
	}
	C.jack_on_shutdown(
		c.jackClient,
		(*[0]byte)(C.jackCShutdownFun),
		unsafe.Pointer(jsc),
	)
}

func (c *Client) SampleRate() uint32 {
	return uint32(C.jack_get_sample_rate(c.jackClient))
}

func (c *Client) registerPort(name string, pType *C.char, flag C.ulong, isTerminal bool, appendTo *[]*Port) (*Port, error) {
	pn := C.CString(name)
	c.pushCString(pn)
	if isTerminal {
		flag |= C.JackPortIsTerminal
	}

	p := C.jack_port_register(c.jackClient, pn, pType, flag, 0)
	if nil == p {
		return nil, fmt.Errorf("no more JACK ports available")
	}

	pObj := newPort(p)
	if nil != appendTo {
		*appendTo = append(*appendTo, pObj)
	}

	return pObj, nil
}

func (c *Client) RegisterAudioIn(name string, isTerminal bool) (*Port, error) {
	return c.registerPort(name, JACK_DEFAULT_AUDIO_TYPE, C.JackPortIsInput, isTerminal, &c.inPorts)
}

func (c *Client) RegisterAudioOut(name string, isSynthesized bool) (*Port, error) {
	return c.registerPort(name, JACK_DEFAULT_AUDIO_TYPE, C.JackPortIsOutput, isSynthesized, &c.outPorts)
}

func (c *Client) RegisterMidiIn(name string, isTerminal bool) (*MidiPort, error) {
	p, err := c.registerPort(name, JACK_DEFAULT_MIDI_TYPE, C.JackPortIsInput, isTerminal, &c.inPorts)
	return &MidiPort{*p, make(map[*MidiCallback]struct{})}, err
}

func (c *Client) Activate() error {
	out := C.jack_activate(c.jackClient)
	if 0 != out {
		return fmt.Errorf(C.GoString(C.strerror(out)))
	}
	return nil
}

func (c *Client) getPorts(flag C.ulong, isPhysical bool) ([]*Port, error) {
	if isPhysical {
		flag |= C.JackPortIsPhysical
	}
	names := convCStrArr(C.jack_get_ports(c.jackClient, nil, nil, flag))
	ps := []*Port{}
	for _, p := range names {
		if nil == p {
			continue
		}
		p2, err := newPortByName(c.jackClient, p)
		if nil != err {
			return nil, err
		}
		ps = append(ps, p2)
	}
	return ps, nil
}

func (c *Client) SystemOutputPorts(isPhysical bool) ([]*Port, error) {
	return c.getPorts(C.JackPortIsOutput, isPhysical)
}

func (c *Client) SystemInputPorts(isPhysical bool) ([]*Port, error) {
	return c.getPorts(C.JackPortIsInput, isPhysical)
}

func (c *Client) OutputPorts() []*Port {
	return c.outPorts
}

func (c *Client) InputPorts() []*Port {
	return c.inPorts
}

func (c *Client) Connect(p1, p2 *Port) error {
	n1, err := p1.name()
	if nil != err {
		return err
	}
	n2, err := p2.name()
	if nil != err {
		return err
	}

	e := C.jack_connect(c.jackClient, n1, n2)
	if C.EEXIST == e {
		return fmt.Errorf("connection exists already")
	} else if e != 0 {
		return fmt.Errorf(C.GoString(C.strerror(e)))
	}

	return nil
}

func (c *Client) pushCString(str *C.char) {
	c.cStrings = append(c.cStrings, str)
}
