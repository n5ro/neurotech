package main

import (
	"encoding/json"
	"time"

	"github.com/kataras/iris/websocket"
	"github.com/mjibson/go-dsp/fft"
)

type Sample struct {
	Name           string
	Channels       []float64
	Timestamp      time.Time
	SequenceNumber uint
}

type FFTData struct {
	Name           string
	Channels       [][]float64
	Timestamp      time.Time
	SequenceNumber uint
}

type WebsocketTunnel struct {
	cliconnections []websocket.Connection
}

func (wst *WebsocketTunnel) HandleEeg(c websocket.Connection) {
	LOG.Infof("websocket connection established with identifier: %s\n", c.ID())
	rawlistener := make(chan []byte, 256)
	go wst.fftloop(rawlistener)
	c.OnDisconnect(func() {
		LOG.Infof("websocket connection closed with identifer: %s\n", c.ID())
	})
	c.OnError(func(err error) {
		LOG.Errorf("websocket connection error with identifier: %s\t[%s]\n", c.ID(), err)
	})
	c.OnMessage(func(data []byte) {
		// wst.broadcast(data)
		rawlistener <- data
	})
}

func (wst *WebsocketTunnel) HandleCli(c websocket.Connection) {
	LOG.Infof("websocket connection established with identifier: %s\n", c.ID())
	c.OnDisconnect(func() {
		LOG.Infof("websocket connection closed with identifer: %s\n", c.ID())
	})
	c.OnError(func(err error) {
		LOG.Errorf("websocket connected error with identifier: %s\t%s\n", c.ID(), err)
	})
	wst.cliconnections = append(wst.cliconnections, c)
}

func (wst *WebsocketTunnel) broadcast(data []byte) {
	for _, clic := range wst.cliconnections {
		go clic.EmitMessage(data)
	}
}

func (wst *WebsocketTunnel) fftloop(rawlistener chan []byte) {
	// assumes b.numchan == 2
	numchan := 2
	ctr := 0
	var seqnum uint
	fftsize := 256
	fftdata0 := make([]float64, fftsize)
	fftdata1 := make([]float64, fftsize)
	for {
		s := <-rawlistener
		sample := &Sample{}
		err := json.Unmarshal(s, sample)
		if err != nil {
			LOG.Errorf("error unmarshalling sample: %s\n", err)
		}
		fftdata0[ctr%fftsize] = sample.Channels[0]
		fftdata1[ctr%fftsize] = sample.Channels[1]
		// Set the frequency that the FFT is sent out.
		// e.g. ctr%2==0, every other sample
		//      ctr%10==0, every 10th sample
		//      ctr%250==0, every 250th sample
		if ctr%16 == 0 {
			fftd := FFTData{
				Name:           "fft",
				Channels:       make([][]float64, numchan),
				SequenceNumber: seqnum,
				Timestamp:      time.Now(),
			}
			fftd.Channels[0] = abs(fft.FFTReal(fftdata0))[:125]
			fftd.Channels[1] = abs(fft.FFTReal(fftdata1))[:125]
			jsonfft, err := json.Marshal(fftd)
			if err != nil {
				LOG.Errorf("error marshalling fft data: %s\n", err)
				continue
			}
			wst.broadcast(jsonfft)
			seqnum++
		}
		ctr++

	}
}
