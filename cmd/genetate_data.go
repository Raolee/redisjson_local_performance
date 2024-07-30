package main

import "math/rand"

type Item struct {
	Field1  string `json:"field1"`
	Field2  string `json:"field2"`
	Field3  string `json:"field3"`
	Field4  string `json:"field4"`
	Field5  string `json:"field5"`
	Field6  string `json:"field6"`
	Field7  string `json:"field7"`
	Field8  string `json:"field8"`
	Field9  string `json:"field9"`
	Field10 string `json:"field10"`
}

func generateField(size int) string {
	runes := make([]rune, size)
	for i := range runes {
		// 대문자와 소문자 알파벳 범위에서 랜덤 문자 생성
		if rand.Intn(2) == 0 {
			runes[i] = rune('A' + rand.Intn(26)) // 대문자
		} else {
			runes[i] = rune('a' + rand.Intn(26)) // 소문자
		}
	}
	return string(runes)
}

type MaybeItemDataSize int

const (
	Maybe10byte  = MaybeItemDataSize(10)
	Maybe100byte = MaybeItemDataSize(100)
	Maybe500byte = MaybeItemDataSize(500)
	Maybe1kb     = MaybeItemDataSize(1000)
	Maybe10kb    = MaybeItemDataSize(10000)
	Maybe100kb   = MaybeItemDataSize(100000)
)

type sizeDispenser struct {
	remainSize int
	windowSize int
}

func (s sizeDispenser) dispense() int {
	if s.remainSize-s.windowSize > 0 {
		s.remainSize -= s.windowSize
		return s.windowSize
	}
	if s.remainSize > 0 {
		return s.remainSize
	}
	return 1 // 최소 size 1은 보장
}

func generateLargeJSON(itemDataSize MaybeItemDataSize) Item {

	dispenser := sizeDispenser{
		remainSize: int(itemDataSize),
		windowSize: int(itemDataSize) / 10,
	}

	// [Note] : 아래 데이털를 json 으로 바꾸면 '{', '}', '"' 와 같은 문자가 추가되므로 itemDataSize 를 무조건 초과하는 데이터가 만들어진다.
	return Item{
		Field1:  generateField(dispenser.dispense()),
		Field2:  generateField(dispenser.dispense()),
		Field3:  generateField(dispenser.dispense()),
		Field4:  generateField(dispenser.dispense()),
		Field5:  generateField(dispenser.dispense()),
		Field6:  generateField(dispenser.dispense()),
		Field7:  generateField(dispenser.dispense()),
		Field8:  generateField(dispenser.dispense()),
		Field9:  generateField(dispenser.dispense()),
		Field10: generateField(dispenser.dispense()),
	}
}
