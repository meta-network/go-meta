// This file is part of the go-meta library.
//
// Copyright (C) 2017 JAAK MUSIC LTD
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
//
// If you have any questions please contact yo@jaak.io

package cwr

import (
	"bufio"
	"io"
	"strings"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/meta-network/go-meta"
)

// Converter converts CWR data from a CWR file to META
// objects.
type Converter struct {
	store *meta.Store
}

type recordJob struct {
	record *Record
	index  int
}

type objectResult struct {
	obj   *meta.Object
	index int
	err   error
}

var concurrentWorkNum = 16

// NewConverter returns a Converter which reads data from the given CWR io.Reader
// and stores META object in the given META store.
func NewConverter(store *meta.Store) *Converter {

	return &Converter{
		store: store,
	}
}

// Transaction struct
type Transaction struct { // NWR,REV,EXC,ACK,AGR or ISW transaction
	//Each transaction include one main record and few details records.
	//a transaction can include few details records from the same type
	MainRecord    map[string]*cid.Cid   `json:"MainRecord"` //NWR,REV,EXC,ACK,AGR or ISW record
	DetailRecords map[string][]*cid.Cid `json:"DetailRecords"`
}

// Group struct
type Group struct {
	Record       *cid.Cid                 `json:"GRH"`          //Group Header
	Transactions map[string][]Transaction `json:"Transactions"` //NWR,REV,EXC,ACK,AGR or ISW transacations
}

// Cwr struct
type Cwr struct {
	Records map[string]*cid.Cid `json:"Records"` //HDR/TRL
	Groups  []Group             `json:"Groups"`  //Each group is a map of transacations
}

// ConvertCWR converts the given source CWR file into a META object graph and
// returns the CID of the graph's root META object.
func (c *Converter) ConvertCWR(cwrFileReader io.Reader) (*cid.Cid, error) {

	jobs := make(chan recordJob)
	results := make(chan objectResult)
	//Due to the concurrency meta objects encoding and the need to keep the order of the cwr records
	//for proper analysys of the cwr each job is indexed and then beeing collected in a recordObjs map where
	//the key is the index of the meta object.
	recordObjs := make(map[int]*meta.Object)

	var cwr Cwr
	cwr.Records = make(map[string]*cid.Cid)
	var nwr Transaction
	nwr.MainRecord = make(map[string]*cid.Cid)
	nwr.DetailRecords = make(map[string][]*cid.Cid)
	var spus []*cid.Cid
	var group Group

	var wg sync.WaitGroup
	wg.Add(concurrentWorkNum)
	for i := 0; i < concurrentWorkNum; i++ {
		go func() {
			defer wg.Done()
			c.worker(jobs, results)
		}()
	}

	go func() {
		scanner := bufio.NewScanner(cwrFileReader)
		index := 0

		for scanner.Scan() {
			record, err := newRecord(scanner.Text())
			if err != nil {
				results <- objectResult{nil, 0, err}
				break
			}
			if record != nil {
				jobs <- recordJob{record, index}
				index++
			}
		}
		if err := scanner.Err(); err != nil {
			results <- objectResult{nil, 0, err}
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var err error = nil
	for v := range results {
		if v.err != nil {
			err = v.err //get the err and continue to drain the channel
			continue
		}
		recordObjs[v.index] = v.obj
	}

	if err != nil {
		return nil, err
	}
	//Itererate over recordsObjs map
	//The keys at the recordsObjs map indicate the order of the meta.objects which were
	//encoded concurrently .
	for i := 0; i < len(recordObjs); i++ {
		obj := recordObjs[i]
		recordType, err := obj.GetString("record_type")
		if err != nil {
			return nil, err
		}
		switch recordType {
		case "HDR", "TRL":
			cwr.Records[recordType] = obj.Cid()
		case "GRH":
			// txs = make(map[string][]Transaction)
			group.Record = obj.Cid()
			group.Transactions = make(map[string][]Transaction)
		case "GRT":
			nwr.DetailRecords["SPU"] = spus // accumulate last spus
			spus = []*cid.Cid{}
			group.Transactions["NWR"] = append(group.Transactions["NWR"], nwr) // accumulate last transaction
			nwr.MainRecord = make(map[string]*cid.Cid)
			nwr.DetailRecords = make(map[string][]*cid.Cid)
			//accumulate txs and continue
			cwr.Groups = append(cwr.Groups, group)
		case "NWR", "REV":
			if nwr.MainRecord[recordType] != nil { // accumulate the current nwr and continue
				nwr.DetailRecords["SPU"] = spus // accumulate spus
				spus = []*cid.Cid{}
				group.Transactions["NWR"] = append(group.Transactions["NWR"], nwr)
				nwr.MainRecord = make(map[string]*cid.Cid)
				nwr.DetailRecords = make(map[string][]*cid.Cid)
			}
			nwr.MainRecord[recordType] = obj.Cid()
		case "SPU": // NWR/REV transaction records
			spus = append(spus, obj.Cid())
		}
	}
	obj, err := meta.Encode(cwr)
	if err != nil {
		return nil, err
	}

	if err := c.store.Put(obj); err != nil {
		return nil, err
	}
	return obj.Cid(), nil
}

func newRecord(line string) (*Record, error) {
	//TODO Add validity check and return errors accordingly.
	record := &Record{}
	switch substring(line, 0, 3) {
	case "HDR":
		record.RecordType = substring(line, 0, 3)
		record.SenderType = substring(line, 3, 6)
		record.SenderID = substring(line, 6, 14)
		record.SenderName = strings.TrimSpace(substring(line, 14, 59))
	case "GRH":
		record.RecordType = substring(line, 0, 3)
		record.TransactionType = substring(line, 3, 6)
		record.GroupID = substring(line, 6, 11)
	case "GRT":
		record.RecordType = substring(line, 0, 3)
		record.GroupID = substring(line, 3, 8)
	case "TRL":
		record.RecordType = substring(line, 0, 3)
	case "NWR", "REV":
		record.RecordType = substring(line, 0, 3)
		record.TransactionSequenceN = substring(line, 3, 12)
		record.RecordSequenceN = substring(line, 12, 19)
		record.Title = strings.TrimSpace(substring(line, 19, 79))
		record.LanguageCode = substring(line, 79, 81)
		record.SubmitteWorkNumber = substring(line, 81, 95)
		record.ISWC = substring(line, 95, 106)
		record.CopyRightDate = substring(line, 106, 113)
		record.DistributionCategory = substring(line, 127, 129)
		record.Duration = substring(line, 129, 135)
		record.RecordedIndicator = substring(line, 135, 136)
		record.TextMusicRelationship = substring(line, 136, 139)
		record.CompositeType = substring(line, 140, 142)
		record.VersionType = substring(line, 142, 145)
		record.PriorityFlag = substring(line, 259, 260)
	case "SPU":
		record.RecordType = substring(line, 0, 3)
		record.TransactionSequenceN = substring(line, 3, 12)
		record.RecordSequenceN = substring(line, 12, 19)
		record.PublisherSequenceNumber = substring(line, 19, 21)
	default:
		return nil, nil
	}
	return record, nil
}

func (c *Converter) worker(jobs <-chan recordJob, results chan<- objectResult) {
	//defer wg.Done()
	for job := range jobs {
		if job.record.RecordType != "" {
			obj, err := meta.Encode(job.record)
			if err != nil {
				results <- objectResult{nil, 0, err}
				return
			}
			if err := c.store.Put(obj); err != nil {
				results <- objectResult{nil, 0, err}
				return
			}
			results <- objectResult{obj, job.index, nil}
		}
	}
}

func substring(s string, from int, to int) string {
	if len(s) < from || len(s) < to {
		return ""
	}
	return s[from:to]
}
