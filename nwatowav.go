package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"github.com/go-rlvm/nwa"
	"io"
	"log"
	"os"
	"strings"
)

var inputfile = flag.String("inputfile", "", "path to the input file.")

type fileType int

const (
	NONE  fileType = iota
	NWA
	NWK
	OVK
)

func main() {
	flag.Parse()

	if *inputfile == "" {
		log.Fatal("You need to define an input file!")
	}

	file, err := os.Open(*inputfile)
	defer file.Close()
	if err != nil {
		log.Fatal(err)
	}

	var outfilename, outext, outpath string
	var filetype fileType
	var headblksz int64

	switch {
	case strings.Contains(*inputfile, ".nwa"):
		{
			filetype = NWA
			outext = "wav"
		}
	case strings.Contains(*inputfile, ".nwk"):
		{
			filetype = NWK
			headblksz = 12
			outext = "wav"
		}
	case strings.Contains(*inputfile, ".ovk"):
		{
			filetype = OVK
			headblksz = 16
			outext = "ogg"
		}
	}
	if filetype == NONE {
		log.Fatal("This program can only handle .nwa/.nwk/.ovk files right now.")
	}

	outfilename = strings.Split(*inputfile, ".")[0]

	if filetype == NWA {
		var data io.Reader
		if data, err = nwa.NewNwaFile(file); err != nil {
			log.Fatal(err)
		}

		outpath = fmt.Sprintf("%s.%s", outfilename, outext)

		var out *os.File
		out, err = os.Create(outpath)
		if err != nil {
			log.Fatal(err)
		}
		defer out.Close()

		if _, err = io.Copy(out, data); err != nil {
			log.Fatal(err)
		}
	} else { // NWK or OVK files
		var indexcount int32
		binary.Read(file, binary.LittleEndian, &indexcount)
		if indexcount <= 0 {
			if filetype == OVK {
				log.Fatalf("Invalid Ogg-ovk file: %s: index = %d\n", inputfile, indexcount)
			} else {
				log.Fatalf("Invalid Koe-nkw file: %s: index = %d\n", inputfile, indexcount)
			}
		}

		tblsiz := make([]int32, indexcount)
		tbloff := make([]int32, indexcount)
		tblcnt := make([]int32, indexcount)
		tblorigsiz := make([]int32, indexcount)

		var i int32
		for i=0;i<indexcount;i++ {
			buffer := new(bytes.Buffer)
			if count, err := io.CopyN(buffer, file, headblksz); count != headblksz || err != nil {
				log.Fatal("Couldn't read the index entries!")
			}
			binary.Read(buffer, binary.LittleEndian, &tblsiz[i])
			binary.Read(buffer, binary.LittleEndian, &tbloff[i])
			binary.Read(buffer, binary.LittleEndian, &tblcnt[i])
			binary.Read(buffer, binary.LittleEndian, &tblorigsiz[i])
		}

		c := make(chan int, indexcount)
		for i=0;i<indexcount;i++ {
			if tbloff[i] <= 0 || tblsiz[i] <= 0 {
				log.Fatalf("Invalid table[%d]: cnt %d, off %d, size %d\n", i, tblcnt[i], tbloff[i], tblsiz[i])
				continue
			}
			buffer := new(bytes.Buffer)
			file.Seek(int64(tbloff[i]), 0)
			if count, err := io.CopyN(buffer, file, int64(tblsiz[i])); count != int64(tblsiz[i]) || err != nil {
				log.Fatalf("Couldn't read the data for table[%d]: cnt %d, off %d, size %d\n", i, tblcnt[i], tbloff[i], tblsiz[i])
			}
			outpath = fmt.Sprintf("%s-%d.%s", outfilename, tblcnt[i], outext)
			go doDecode(filetype, outpath, buffer, c)
		}
		for i=0;i<indexcount;i++ {
			<-c
		}
	}
}

func doDecode(filetype fileType, filename string, data io.Reader, c chan int) {
	var err error
	if filetype == NWK {
		if data, err = nwa.NewNwaFile(data); err != nil {
					log.Fatal(err)
		}
	}
	var out *os.File
	out, err = os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()
	if _, err = io.Copy(out, data); err != nil {
		log.Fatal(err)
	}
	c<-1
}
