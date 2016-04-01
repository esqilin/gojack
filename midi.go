package gojack

// #include <jack/jack.h>
// #include <jack/midiport.h>
import "C"
import "unsafe"

const (
	MIDI_NOTE_ON  = 0x90
	MIDI_NOTE_OFF = 0x80
)

var JACK_DEFAULT_MIDI_TYPE *C.char = C.CString(C.JACK_DEFAULT_MIDI_TYPE)

type MidiCallback func(byte, byte, byte)

type MidiPort struct {
	Port
	callbacks map[*MidiCallback]struct{}
}

func (c MidiPort) ProcessEvents(nFrames int) {
	var e C.jack_midi_event_t
	buf := C.jack_port_get_buffer(c.Port.p, C.jack_nframes_t(nFrames))
	n := int(C.jack_midi_get_event_count(buf))
	for i := 0; i < n; i++ {
		C.jack_midi_event_get(&e, buf, C.uint32_t(i))
		//~ j := int(e.time)
		//~ m[j] = append(m[j], newMidiEvent(e.buffer))
		params := C.GoBytes(unsafe.Pointer(e.buffer), 3)
		for c, _ := range c.callbacks {
			(*c)(params[0], params[1], params[2])
		}
	}
}

func (mp *MidiPort) AddCallback(c *MidiCallback) {
	mp.callbacks[c] = struct{}{}
}

func (mp *MidiPort) RemoveCallback(c *MidiCallback) {
	delete(mp.callbacks, c)
}
