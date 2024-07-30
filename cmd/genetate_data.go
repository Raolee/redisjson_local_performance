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
	Field11 string `json:"field11"`
	Field12 string `json:"field12"`
	Field13 string `json:"field13"`
	Field14 string `json:"field14"`
	Field15 string `json:"field15"`
	Field16 string `json:"field16"`
	Field17 string `json:"field17"`
	Field18 string `json:"field18"`
	Field19 string `json:"field19"`
	Field20 string `json:"field20"`
	Field21 string `json:"field21"`
	Field22 string `json:"field22"`
	Field23 string `json:"field23"`
	Field24 string `json:"field24"`
	Field25 string `json:"field25"`
	Field26 string `json:"field26"`
	Field27 string `json:"field27"`
	Field28 string `json:"field28"`
	Field29 string `json:"field29"`
	Field30 string `json:"field30"`
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

type ItemDataSize int

const (
	ItemDataSize_10byte  = ItemDataSize(10)
	ItemDataSize_100byte = ItemDataSize(100)
	ItemDataSize_500byte = ItemDataSize(500)
	ItemDataSize_1kb     = ItemDataSize(1000)
	ItemDataSize_10kb    = ItemDataSize(10000)
	ItemDataSize_100kb   = ItemDataSize(100000)
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
	defer func() {
		s.remainSize = 0
	}()
	return s.remainSize
}

func generateLargeJSON(itemDataSize ItemDataSize) Item {

	dispenser := sizeDispenser{
		remainSize: int(itemDataSize) * 30,
		windowSize: int(itemDataSize) / 30,
	}

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
		Field11: generateField(dispenser.dispense()),
		Field12: generateField(dispenser.dispense()),
		Field13: generateField(dispenser.dispense()),
		Field14: generateField(dispenser.dispense()),
		Field15: generateField(dispenser.dispense()),
		Field16: generateField(dispenser.dispense()),
		Field17: generateField(dispenser.dispense()),
		Field18: generateField(dispenser.dispense()),
		Field19: generateField(dispenser.dispense()),
		Field20: generateField(dispenser.dispense()),
		Field21: generateField(dispenser.dispense()),
		Field22: generateField(dispenser.dispense()),
		Field23: generateField(dispenser.dispense()),
		Field24: generateField(dispenser.dispense()),
		Field25: generateField(dispenser.dispense()),
		Field26: generateField(dispenser.dispense()),
		Field27: generateField(dispenser.dispense()),
		Field28: generateField(dispenser.dispense()),
		Field29: generateField(dispenser.dispense()),
		Field30: generateField(dispenser.dispense()),
	}
}
