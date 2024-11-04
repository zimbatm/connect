package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"time"

	"bringyour.com/inspect/evaluation"
	"bringyour.com/inspect/grouping"
	"bringyour.com/inspect/payload"
	"bringyour.com/protocol"

	"github.com/oklog/ulid/v2"
	"github.com/shenwei356/countminsketch"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
	"google.golang.org/protobuf/proto"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: 'go run . MODE' where MODE={p,dt,t,c,e,ghc,st,rc}")
	}
	fname := os.Args[1]

	// File paths for original data, transport records and cooccurence matrix
	// testSession := evaluation.TestSession1
	testSession := evaluation.TestSession1
	fmt.Printf("TestSession=%+v\n", testSession)

	// CLUSTERING OPTIONS (currently not used for main clustering since we are using go clustering)
	// clusterMethod := grouping.NewOptics(fmt.Sprintf("min_samples=%d,max_eps=%f", 3, 0.20227))
	clusterMethod := grouping.NewHDBSCAN(fmt.Sprintf("min_cluster_size=%d,min_samples=%d,cluster_selection_epsilon=%.12f,alpha=%.12f", 4, 1, 0.001, 0.001))

	// OVERLAP FUNCTIONS
	overlapFunctions := grouping.FixedMarginOverlap{
		Margin: grouping.TimestampInNano(0.010_000_000), // x seconds fixed margin
	}
	// overlapFunctions := grouping.GaussianOverlap{
	// 	StdDev: grouping.TimestampInNano(0.010_000_000), // x seconds
	// 	Cutoff: 4,                                       // x standard deviations
	// }

	if fname == "parse_pcap" || fname == "p" {
		// create transport records from pcap and save them to file (source IP is needed to know which direction is incoming)
		payload.PcapToTransportFiles(testSession.DataPath, testSession.SavePath, testSession.SourceIP)
	} else {
		if fname == "cluster" || fname == "c" {
			// cluster based on a cooccurrence matrix
			runCluster(clusterMethod, testSession.CoOccurrencePath)
			return
		} else if fname == "reduce_cooc" || fname == "rc" {
			// reduce cooccurrence matrix using count-min sketch
			testReduceCooc(testSession.CoOccurrencePath)
			return
		}

		// load transport records from file
		records, err := payload.LoadTransportsFromFiles(testSession.SavePath)
		if err != nil {
			log.Fatalf("Error loading transports: %v", err)
		}

		if fname == "display_transports" || fname == "dt" {
			// print transport records stored in a file
			payload.DisplayTransports(records)
		} else if fname == "timestamps" || fname == "t" {
			// build cooccurrence map from transport records
			parseTimestamps(&overlapFunctions, records, testSession.CoOccurrencePath)
		} else if fname == "evaluate" || fname == "e" {
			// build cooccurrence map, cluster and evaluate
			runEvaluate(&overlapFunctions, clusterMethod, records, testSession.CoOccurrencePath, testSession.Regions)
		} else if fname == "genetic_hill_climbing" || fname == "ghc" {
			// run genetic hill climbing to try and find the best options for cluster method (uses python clustering)
			evaluation.GeneticHillClimbing(records, testSession)
		} else if fname == "save_times" || fname == "st" {
			// save the times of transport records to a file (used for analysis in python)
			saveTimes(&overlapFunctions, records, testSession.TimesPath)
		} else {
			log.Fatalf("Unknown mode: %s. Available ones are {p,dt,t,c,e,ghc,st,rc}", fname)
		}
	}
}

// This method is just a test method to see if the count-min sketch method can be used to estimate the overlap values effectively in a fixed matrix size
func testReduceCooc(coOccurrencePath string) {
	// the idea of reducing the cooc map is that while loading the transport records you can
	// use the count-min sketch to estimate the overlap values for the cooc map
	// and store them in a fixed matrix size (8*d*w bytes)
	// in stead of the current sparse matrix (max size is ~16N^2 bytes but on average only 7% are used so around 1.12*N^2 bytes)
	//
	// if it is decided to be used in stead of the current sparse matrix
	// then the MakeCoOccurrence method should be updated to use the count-min sketch to estimate the overlap values
	// and the underlying implementation of the coOccurence map should be updated to use a fixed matrix
	// additionally, the cooc protobuf types should be changed and the save and load functions

	// load cooc data from file (normally this should be done while loading transport records and not cooc map)
	cooc := grouping.NewCoOccurrence(nil)
	if err := cooc.LoadData(coOccurrencePath); err != nil {
		panic(err)
	}
	// get size of current (sparse) cooc map
	dataSize, idMapSize := cooc.MemorySize()
	log.Printf("CoOccurrence memory size: data=%d ids=%d total=%d bytes", dataSize, idMapSize, dataSize+idMapSize)

	d := uint(12)                    // number of hash functions
	w := uint(1000)                  // number of counters per hash function
	s, _ := countminsketch.New(d, w) // size of data is 8*d*w bytes
	fmt.Printf("d: %d, w: %d\n", s.D(), s.W())

	// go through cooc data and add to count min sketch (normally this should be done while loading the transport records)
	total, wrong := 0, 0
	for i, cmap := range *cooc.GetData() {
		for j, v := range cmap {
			s.UpdateString(fmt.Sprintf("%d-%d", i, j), v)
		}
	}

	coocData := make([]*protocol.CoocOuter, 0) // save estimated values to protobuf object (if you want to use them for clustering or to visualize heatmap)
	totalErr := 0.0

	// compare estimated (count min sketch) value to one in (sparse) cooc map
	for i, coocInner := range *cooc.GetData() {
		outer := &protocol.CoocOuter{Cid: i}
		for j, v := range coocInner {
			estimate := s.EstimateString(fmt.Sprintf("%d-%d", i, j))                                  // load estimated value from count min sketch
			outer.CoocInner = append(outer.CoocInner, &protocol.CoocInner{Cid: j, Overlap: estimate}) // save estimate to protobuf object
			total++
			if estimate != v {
				wrong++
				currErr := float64(estimate-v) / float64(v) * 100 // error in percentage
				totalErr += currErr
				// fmt.Printf("Estimate failed for %d-%d: %d!=%d (difference=%d overflow by %.2f%%)\n", i, j, estimate, v, estimate-v, currErr)
			}
		}
		coocData = append(coocData, outer)
	}
	fmt.Printf("Total: %d, Wrong: %d, AvgErr: %.2f%%\n", total, wrong, totalErr/float64(wrong))

	// get size of count min sketch using WriteTo to compare to sparse matrix size
	buf := new(bytes.Buffer)
	size, _ := s.WriteTo(buf)
	log.Printf("CountMinSketch memory size: %d bytes", size)
	buf = nil

	mappingData := make([]*protocol.CoocSid, 0)
	for sid, cid := range *cooc.GetInternalMapping() {
		mappingData = append(mappingData, &protocol.CoocSid{Sid: string(sid), Cid: cid})
	}
	dataToSave := &protocol.CooccurrenceData{CoocOuter: coocData, CoocSid: mappingData}
	_, err := proto.Marshal(dataToSave)
	// out, err := proto.Marshal(dataToSave)
	if err != nil {
		panic(err)
	}

	// UNCOMMENT IF YOU WANT TO SAVE IN FILE
	// save cooc to file if you want to cluster it
	// err = os.WriteFile(coOccurrencePath, out, 0644)
	// if err != nil {
	// 	panic(err)
	// }
}

func parseTimestamps(overlapFunctions grouping.OverlapFunctions, records *map[ulid.ULID]*payload.TransportRecord, coOccurrencePath string) {
	// build cooccurrence map
	sessionTimestamps := grouping.MakeTimestamps(overlapFunctions, records)
	cooc, _ := grouping.MakeCoOccurrence(sessionTimestamps)
	cooc.SaveData(coOccurrencePath)
	dataSize, idMapSize := cooc.MemorySize()
	log.Printf("CoOccurrence memory size: data=%d ids=%d total=%d bytes", dataSize, idMapSize, dataSize+idMapSize)
	overlapStats(cooc)
}

func runCluster(clusterMethod grouping.ClusterMethod, coOccurrencePath string) {
	// cluster
	clusterOps := &grouping.ClusterOpts{
		ClusterMethod:    clusterMethod,
		CoOccurrencePath: coOccurrencePath,
		SaveGraphs:       true,
		ShowHeatmapStats: true,
	}
	clusters, probabilities, err := grouping.Cluster(clusterOps, true)
	if err != nil {
		log.Fatalf("Error clustering: %v", err)
	}

	for clusterID, probs := range probabilities {
		fmt.Printf("\nCluster %s (len=%d):\n", clusterID, len(clusters[clusterID]))
		avgProb := 0.0
		for i, sid := range clusters[clusterID] {
			fmt.Printf("  %s: %.3f\n", sid, probs[i])
			avgProb += probs[i]
		}
		if len(clusters[clusterID]) > 0 {
			avgProb /= float64(len(clusters[clusterID]))
			fmt.Printf("  Average probability: %.3f\n", avgProb)
		}
	}
}

func runEvaluate(overlapFunctions grouping.OverlapFunctions, clusterMethod grouping.ClusterMethod, records *map[ulid.ULID]*payload.TransportRecord, coOccurrencePath string, initialRegions []evaluation.Region) {
	time1 := time.Now()
	// build cooccurrence map
	sessionTimestamps := grouping.MakeTimestamps(overlapFunctions, records)
	cooc, earliestTimestamp := grouping.MakeCoOccurrence(sessionTimestamps)
	cooc.SaveData(coOccurrencePath)
	time1end := time.Since(time1)

	// cluster
	clusterOps := &grouping.ClusterOpts{
		ClusterMethod:    clusterMethod,
		CoOccurrencePath: coOccurrencePath,
		SaveGraphs:       true,
	}
	time2 := time.Now()
	clusters, probabilities, err := grouping.Cluster(clusterOps, true)
	if err != nil {
		log.Fatalf("Error clustering: %v", err)
	}
	time2end := time.Since(time2)
	for clusterID, probs := range probabilities {
		fmt.Printf("\nCluster %s (len=%d):\n", clusterID, len(clusters[clusterID]))
		avgProb := 0.0
		for i, sid := range clusters[clusterID] {
			fmt.Printf("  %s: %.3f\n", sid, probs[i])
			avgProb += probs[i]
		}
		if len(clusters[clusterID]) > 0 {
			avgProb /= float64(len(clusters[clusterID]))
			fmt.Printf("  Average probability: %.3f\n", avgProb)
		}
	}

	time3 := time.Now()
	// evaluate
	regions := evaluation.ConstructRegions(initialRegions, earliestTimestamp, 3)
	// for i, r := range *regions {
	// 	fmt.Printf("Region %d: %s - %s\n", i+1, ReadableTime(r.minT), ReadableTime(r.maxT))
	// }
	score := evaluation.Evaluate(*sessionTimestamps, *regions, clusters, probabilities)
	time3end := time.Since(time3)
	log.Printf("Score: %f", score)

	fmt.Printf("Time to build cooccurrence map: %v\n", time1end)
	fmt.Printf("Time to cluster(+heatmap): %v\n", time2end)
	fmt.Printf("Time to evaluate: %v\n", time3end)
}

func saveTimes(overlapFunctions grouping.OverlapFunctions, records *map[ulid.ULID]*payload.TransportRecord, timesPath string) {
	// build cooccurrence map
	sessionTimestamps := grouping.MakeTimestamps(overlapFunctions, records)

	timesData := make([]*protocol.Times, 0)
	for sid, timestamps := range *sessionTimestamps {
		times := protocol.Times{
			Sid:  string(sid),
			Time: timestamps.Times,
		}
		timesData = append(timesData, &times)
	}

	dataToSave := &protocol.TimesData{
		Times: timesData,
	}

	out, err := proto.Marshal(dataToSave)
	if err != nil {
		panic(err)
	}

	if err := os.WriteFile(timesPath, out, 0644); err != nil {
		panic(err)
	}
}

// print statistics about overlaps in cooccurrence map
func overlapStats(cooc *grouping.CoOccurrence) {
	float64Overlaps := make([]float64, 0)
	for _, v := range *cooc.GetNonZeroData() {
		float64Overlaps = append(float64Overlaps, grouping.TimestampInSeconds(v))
	}
	log.Printf(`Co-occurrence statistics:
# of timestamps: %d
# non-zero overlaps: %d
	Min: %.9f
	Max: %.9f
	Mean: %.9f
	StdDev: %.9f`,

		len(*cooc.GetInternalMapping()),
		len(float64Overlaps),
		floats.Min(float64Overlaps),
		floats.Max(float64Overlaps),
		stat.Mean(float64Overlaps, nil),
		stat.StdDev(float64Overlaps, nil),
	)
}
