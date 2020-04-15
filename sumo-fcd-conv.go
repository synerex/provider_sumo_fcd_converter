package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"

	pagent "github.com/synerex/proto_people_agent"
	sxapi "github.com/synerex/synerex_api"
	sxproto "github.com/synerex/synerex_proto"
	sxutil "github.com/synerex/synerex_sxutil"
)

var (
	nodesrv         = flag.String("nodesrv", "127.0.0.1:9990", "Node ID Server")
	speed           = flag.Int("speed", 50, "Wait speed millisec(default 50 msec)")
	in              = flag.String("in", "output.xml", "Input XML FCD file (default output.xml)")
	mu              sync.Mutex
	version         = "0.01"
	sxServerAddress string
)

var _ = time.Parse

type DurationType time.Duration

// FcdExport is generated from an XSD element
type FcdExport struct {
	Timesteps []Timestep   `xml:"timestep,omitempty"`
	I         noExtraInner `xml:",any"`
}

// Timestep is generated from an XSD element
type Timestep struct {
	TimeData float32      `xml:"time,attr"`
	Vehicles []Vehicle    `xml:"vehicle,omitempty"`
	Persons  []Person     `xml:"person,omitempty"`
	I        noExtraInner `xml:",any"`
}

// Vehicle is generated from an XSD element
type Vehicle struct {
	ID              string       `xml:"id,attr"`
	X               float64      `xml:"x,attr"`
	Y               float64      `xml:"y,attr"`
	Z               float64      `xml:"z,omitempty,attr"`
	Angle           float64      `xml:"angle,attr"`
	Type            string       `xml:"type,attr"`
	Speed           float32      `xml:"speed,attr"`
	Pos             float32      `xml:"pos,attr"`
	Lane            string       `xml:"lane,omitempty,attr"`
	Slope           float32      `xml:"slope,attr"`
	Signals         int          `xml:"signals,omitempty,attr"`
	Distance        float32      `xml:"distance,omitempty,attr"`
	Acceleration    float32      `xml:"acceleration,omitempty,attr"`
	AccelerationLat float32      `xml:"accelerationLat,omitempty,attr"`
	I               noExtraInner `xml:",any"`
}

type Person struct {
	ID              string       `xml:"id,attr"`
	X               float64      `xml:"x,attr"`
	Y               float64      `xml:"y,attr"`
	Z               float64      `xml:"z,omitempty,attr"`
	Angle           float64      `xml:"angle,attr"`
	Type            string       `xml:"type,attr"`
	Speed           float32      `xml:"speed,attr"`
	Pos             float32      `xml:"pos,attr"`
	Lane            string       `xml:"lane,omitempty,attr"`
	Slope           float32      `xml:"slope,attr"`
	Signals         int          `xml:"signals,omitempty,attr"`
	Distance        float32      `xml:"distance,omitempty,attr"`
	Acceleration    float32      `xml:"acceleration,omitempty,attr"`
	AccelerationLat float32      `xml:"accelerationLat,omitempty,attr"`
	I               noExtraInner `xml:",any"`
}

type noExtraInner struct{}

func (u *noExtraInner) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	return xml.UnmarshalError(fmt.Sprint("detected an extra XML element. (", start.Name.Space, start.Name.Local, ")"))
}

func sendEachTime(fcd *FcdExport, clt *sxutil.SXServiceClient) {
	log.Printf("Len %d", len(fcd.Timesteps))

	agents := make([]*pagent.PAgent, 0, 0)

	scount := 0
	for _, ts := range fcd.Timesteps {
		count := 0
		for _, vs := range ts.Vehicles { // start with Vehicle

			//			var xy = []float64{pt.X, pt.Y}
			//			latlon, _ := proj.Inverse(proj.WebMercator, xy)
			latlon := []float64{vs.X, vs.Y}
			var id int
			ids := strings.Replace(vs.ID, ".", "", -1)

			if strings.HasPrefix(ids, "bus") {
				id, _ = strconv.Atoi(ids[3:])
				//				id = 1000
			}
			if strings.HasPrefix(ids, "f") {
				id, _ = strconv.Atoi(ids[1:])
				id += 1000
			}
			ag := pagent.PAgent{
				Id:    int32(id),
				Point: latlon,
			}
			agents = append(agents, &ag)
			count++
		}
		scount += count
		log.Printf("Agents: %d %d", scount, count)
		pagents := pagent.PAgents{
			Agents: agents,
		}

		out, _ := proto.Marshal(&pagents) // TODO: handle error
		cont := sxapi.Content{Entity: out}
		smo := sxutil.SupplyOpts{
			Name:  "Agents",
			Cdata: &cont,
		}
		_, nerr := clt.NotifySupply(&smo)
		if nerr != nil { // connection failuer with current client
			log.Printf("Connection failure", nerr)
		}

		// sleep ...
		time.Sleep(time.Duration(*speed) * time.Millisecond)

	}
}

func main() {
	log.Printf("SumoFCD Convert Provider(%s) built %s sha1 %s", sxutil.GitVer, sxutil.BuildTime, sxutil.Sha1Ver)
	//	rand.Seed(time.Now().UnixNano())
	flag.Parse()

	go sxutil.HandleSigInt()
	wg := sync.WaitGroup{} // for syncing other goroutines

	sxutil.RegisterDeferFunction(sxutil.UnRegisterNode)
	channelTypes := []uint32{sxproto.PEOPLE_AGENT_SVC}
	srv, rerr := sxutil.RegisterNode(*nodesrv, "SumoFCD", channelTypes, nil)
	if rerr != nil {
		log.Fatal("Can't register node ", rerr)
	}
	log.Printf("Connecting SynerexServer at [%s]\n", srv)

	client := sxutil.GrpcConnectServer(srv)
	sxServerAddress = srv
	argJSON := fmt.Sprintf("{SumoFCD}")

	// parse XML Fleet Car Data

	peopleClient := sxutil.NewSXServiceClient(client, sxproto.PEOPLE_AGENT_SVC, argJSON)
	wg.Add(1)
	log.Printf("Starting SumoFCD Convert Provider")

	f, err := os.Open(*in)
	defer f.Close()

	if err != nil {
		log.Fatal("Can't open XML file")
	}

	xmlBytes, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatal("Can't read XML file")
	}

	data := new(FcdExport)
	if err := xml.Unmarshal(xmlBytes, data); err != nil {
		log.Fatal("XML Unmarshal error:", err)
	}

	sendEachTime(data, peopleClient)

	log.Printf("Done.")
	//	wg.Wait()

}
