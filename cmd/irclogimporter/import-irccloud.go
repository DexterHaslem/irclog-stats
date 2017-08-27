package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"irclog-stats/cfg"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "github.com/lib/pq"
)

var fImportDir = flag.String("dir", "", "directory of log files to import")
var fNetwork = flag.String("network", "", "name of network for logs, eg 'freenode'")
var fConfig = flag.String("cfg", "", "config filename. eg ircimport.json")
var c *cfg.C
var net string

var db *sql.DB

func main() {
	flag.Parse()

	dir := *fImportDir
	net = *fNetwork
	cfgfile := *fConfig
	if cfgfile == "" {
		log.Fatal("config file is required for database connectivity")
	}

	c, err := cfg.From(cfgfile)
	if err != nil {
		log.Fatal(err)
	}

	dbinfo := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable host=%s port=%d",
		c.Username, c.Password, c.DBName, c.Host, c.Port)
	db, err = sql.Open("postgres", dbinfo)

	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	s, err := os.Stat(dir)
	if err != nil {
		log.Fatal(err)
	}

	if !s.IsDir() {
		log.Fatalf("%s is not a directory", s.Name())
	}

	if net == "" {
		log.Fatal("network name is required")
	}

	importDir(dir)
}

func insertLine(cn, line string) {
	// irccloud formats export logs like so
	//[2016-10-03 21:38:29] → Joined channel #warsow.na
	//[2016-10-03 21:38:33] * Channel mode is +tnCNuT
	//[2016-10-03 21:38:45] <dmh> asdfadsf
	// and uses funky symbols for for notices ferried to current channel,
	// joins and parts. just ignore everything that isnt a message.

	// if the very first line of the channel, its just channel name.
	// we already got channel name from filename, ignore it
	if line[0] == '#' {
		return
	}

	// first part, lop off timestamp. this is stable since it will always
	// be the first one
	tsIdx := strings.Index(line, "]")
	if tsIdx == -1 {
		return
	}

	tsChunk := line[1:tsIdx] // leave out the [ ] parts
	restChunks := strings.Split(line[tsIdx+2:], " ")

	if len(restChunks) < 2 {
		return
	}

	// rest chunk 0 = sender name. if its not a nick (so part/join notification etc)
	// ignore it
	if restChunks[0][0] != '<' {
		return
	}
	cleanedNick := strings.Trim(restChunks[0], "<>")
	msg := strings.Join(restChunks[1:], " ")

	//fmt.Printf("c = '%s' - ts = '%s' nick='%s' msg='%s'\n", cn, tsChunk, cleanedNick, msg)
	p, err := db.Prepare("select add_msg((select * from get_network_id($1)), $2::timestamp, $3, $4, $5);")
	if err != nil {
		log.Println(err)
		return
	}
	_, err = p.Exec(net, tsChunk, cleanedNick, cn, msg)
	if err != nil {
		log.Println(err)
	}
}

func importFile(fn string, wg *sync.WaitGroup) {
	defer wg.Done()

	b := filepath.Base(fn)
	fmt.Printf("importing file %s..\n", b)

	count := 0
	f, err := os.Open(fn)
	if err != nil {
		return
	}

	defer f.Close()
	// ITS ENCODED IN UTF-8 BOM header which screws up everything.
	// skip it. however, check in case this changes in future
	tryBom := make([]byte, 3)
	f.Read(tryBom)

	if tryBom[0] == 0xEF && tryBom[1] == 0xBB && tryBom[2] == 0xBF {
		f.Seek(3, 0)
	} else {
		f.Seek(0, 0)
	}

	s := bufio.NewScanner(f)

	for s.Scan() {
		b := filepath.Base(fn)
		ext := filepath.Ext(b)
		// lop the file extension off
		cn := b[0 : len(b)-len(ext)]
		insertLine(cn, s.Text())
		count++
	}
	fmt.Printf("done importing %s, added %d lines\n", b, count)
}

func importDir(dir string) {
	allFiles := []string{}
	filepath.Walk(dir, func(p string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}

		allFiles = append(allFiles, p)
		return nil
	})

	wg := &sync.WaitGroup{}
	wg.Add(len(allFiles))

	for _, f := range allFiles {
		// ok so irccloud notes:
		// network msgs go in a log starting with '-'
		// just skip those.

		// also, only import channels!
		// no PM support

		base := filepath.Base(f)
		if strings.HasPrefix(base, "-") {
			fmt.Printf("skipping network status window log %s\n", base)
			wg.Done()
			continue
		}

		// this will miss weird channels and specials on some ircds.
		if !strings.HasPrefix(base, "#") {
			fmt.Printf("skipping non-channel log file %s\n", base)
			wg.Done()
			continue
		}

		go importFile(f, wg)
	}
	wg.Wait()
}
