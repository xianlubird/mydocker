package network
import(
	"testing"
	"github.com/xianlubird/mydocker/container"
)

func TestBridgeInit(t *testing.T) {
	d := BridgeNetworkDriver{}
	_, err := d.Create("192.168.0.1/24", "testbridge")
	t.Logf("err: %v", err)
}

func TestBridgeConnect(t *testing.T) {
	ep := Endpoint{
		ID: "testcontainer",
	}

	n := Network{
		Name: "testbridge",
	}

	d := BridgeNetworkDriver{}
	err := d.Connect(&n, &ep)
	t.Logf("err: %v", err)
}

func TestNetworkConnect(t *testing.T) {

	cInfo := &container.ContainerInfo{
		Id: "testcontainer",
		Pid: "15438",
	}

	d := BridgeNetworkDriver{}
	n, err := d.Create("192.168.0.1/24", "testbridge")
	t.Logf("err: %v", n)

	Init()

	networks[n.Name] = n
	err = Connect(n.Name, cInfo)
	t.Logf("err: %v", err)
}

func TestLoad(t *testing.T) {
	n := Network{
		Name: "testbridge",
	}

	n.load("/var/run/mydocker/network/network/testbridge")

	t.Logf("network: %v", n)
}
