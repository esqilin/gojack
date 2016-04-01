package gojack

// #include <jack/jack.h>
import "C"
import (
	"fmt"
	"os"
	"unsafe"
)

type processCallback func([][]float32, *[][]float32, interface{}) error
type shutdownCallback func(interface{})

type jackProcessCall struct {
	c    *Client
	fp   *processCallback
	args *interface{}
}

type jackShutdownCall struct {
	fp   *shutdownCallback
	args *interface{}
}

//export jackGoProcessFun
func jackGoProcessFun(nFrames C.jack_nframes_t, args unsafe.Pointer) (status C.int) {
	defer func() {
		r := recover()
		if nil != r {
			fmt.Fprintf(os.Stderr, "error in processing function: %v\n", r)
			status = 3
		}
	}()

	var cArr *C.float
	n := int(nFrames)
	jpc := (*jackProcessCall)(args)

	ps := jpc.c.InputPorts()
	inData := make([][]float32, len(ps))
	for i, p := range ps {
		cArr = (*C.float)(C.jack_port_get_buffer(p.p, nFrames))
		convCFloat32Arr(cArr, n, &inData[i])
	}

	ps = jpc.c.OutputPorts()
	outData := make([][]float32, len(ps))
	for i, p := range ps {
		cArr = (*C.float)(C.jack_port_get_buffer(p.p, nFrames))
		convCFloat32Arr(cArr, n, &outData[i])
	}

	err := (*jpc.fp)(inData, &outData, *jpc.args)
	if nil != err {
		return C.int(1)
	}
	return C.int(0)
}

//export jackGoShutdownFun
func jackGoShutdownFun(args unsafe.Pointer) {
	jsc := (*jackShutdownCall)(args)
	(*jsc.fp)(jsc.args)
}
