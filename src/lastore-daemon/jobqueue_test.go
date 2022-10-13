// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"testing"
	"time"
)

func TestJobQueue(t *testing.T) {
	q := NewJobQueue("test", 1)
	_, err := q.Remove("It should don't exists")
	if err == nil {
		t.Fatal("Remove none exists Job")
	}
	N := 100
	go func() {
		for {
			time.Sleep(time.Millisecond * 10)
			q.AllJobs()
			q.DoneJobs()
		}
	}()

	go func(t *testing.T) {
		for i := 0; i < N; i++ {
			id := fmt.Sprintf("%d", i)
			j := NewJob(nil, id, "test1", nil, id, id, nil)
			err := q.Add(j)
			if err != nil {
				t.Log(err)
			}
		}
		q.PendingJobs()
	}(t)

	for i := 0; i < N; i++ {
		id := fmt.Sprintf("%d", i)
		job := q.Find(id)
		if job == nil {
			continue
		}
		_, err := q.Remove(id)
		if err != nil {
			time.Sleep(time.Millisecond * 10)
			if _, e := q.Remove(id); e != nil {
				t.Fatalf("Remove failed at %d. %v %v", i, e, q.AllJobs())
			}
		}
	}
}
