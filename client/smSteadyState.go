package client

import (
	"log"
	"net"

	"github.com/ProjectNiwl/tinysocks"
	"github.com/bunsim/geph/niaucchi"
)

// smSteadyState represents the steady state of the client.
// => ConnEntry when the network fails.
func (cmd *Command) smSteadyState() {
	log.Println("** => SteadyState **")
	defer log.Println("** <= SteadyState **")
	// change stats
	cmd.stats.Lock()
	cmd.stats.status = "connected"
	cmd.stats.Unlock()
	defer func() {
		cmd.stats.Lock()
		cmd.stats.status = "connecting"
		cmd.stats.Unlock()
	}()
	// spawn the SOCKS5 server
	socksListener, err := net.Listen("tcp", "127.0.0.1:8781")
	if err != nil {
		panic(err.Error())
	}
	go cmd.doSocks(socksListener)
	defer socksListener.Close()
	// wait until death
	reason := cmd.currTunn.Tomb().Wait()
	log.Println("network failed in steady state:", reason.Error())
	// clear everything and go to ConnEntry
	cmd.currTunn = nil
	cmd.smState = cmd.smConnEntry
}

func (cmd *Command) doSocks(lsnr net.Listener) {
	for {
		clnt, err := lsnr.Accept()
		if err != nil {
			return
		}
		go func() {
			defer clnt.Close()
			var myss *niaucchi.Substrate
			myss = cmd.currTunn
			if myss == nil {
				return
			}
			dest, err := tinysocks.ReadRequest(clnt)
			if err != nil {
				return
			}
			conn, err := myss.OpenConn()
			if err != nil {
				return
			}
			defer conn.Close()
			tinysocks.CompleteRequest(0x00, clnt)
			conn.Write([]byte{byte(len(dest))})
			conn.Write([]byte(dest))
			// forward
			log.Println("tunnel open", dest)
			cmd.stats.Lock()
			cmd.stats.netinfo.tuns[clnt.RemoteAddr().String()] = dest
			cmd.stats.Unlock()
			defer log.Println("tunnel clos", dest)
			defer func() {
				cmd.stats.Lock()
				delete(cmd.stats.netinfo.tuns, clnt.RemoteAddr().String())
				cmd.stats.Unlock()
			}()
			go func() {
				defer conn.Close()
				defer clnt.Close()
				ctrCopy(clnt, conn, &cmd.stats.rxBytes)
			}()
			ctrCopy(conn, clnt, &cmd.stats.txBytes)
		}()
	}
}