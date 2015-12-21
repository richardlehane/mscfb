// Copyright 2015 Richard Lehane. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mscfb

import "strconv"

const (
	ErrFormat = iota
	ErrRead
	ErrTraverse
)

type Error struct {
	typ int
	msg string
	val int64
}

func (e Error) Error() string {
	return "mscfb: " + e.msg + "; " + strconv.FormatInt(e.val, 10)
}

func (e Error) Typ() int {
	return e.typ
}
