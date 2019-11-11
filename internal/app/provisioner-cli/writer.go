/*
 * Copyright 2019 Nalej
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package provisioner_cli

import (
	"fmt"
	"os"
	"text/tabwriter"
)

// TabWriterHelper structure to simplify writing on a Tab writer.
type TabWriterHelper struct {
	w   *tabwriter.Writer
	err error
}

// NewTabWriterHelper creates a new writer on stdout
func NewTabWriterHelper() *TabWriterHelper {
	result := &TabWriterHelper{w: new(tabwriter.Writer)}
	// Format in tab-separated columns with a tab stop of 8.
	result.w.Init(os.Stdout, 0, 8, 0, '\t', 0)
	return result
}

// Println tries to print the data on the output writer. This function captures the first error
// so that it is returned if present on the flush operation.
func (twh *TabWriterHelper) Println(data ...interface{}) {
	if twh.err != nil {
		return
	}
	_, twh.err = fmt.Fprintln(twh.w, data...)
}

// Flush finishes the writing operation by sending all remaining items to stdout.
func (twh *TabWriterHelper) Flush() error {
	if twh.err != nil {
		return twh.err
	}
	return twh.w.Flush()
}
