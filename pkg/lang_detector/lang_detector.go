package lang_detector

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	path_util "github.com/pluveto/pastex2/pkg/path_util"
)

// key: language name,
// value: language config
//		key: regexp
//		value: points
var rules map[string](map[*regexp.Regexp]int)

func LoadRules() {
	rules = make(map[string](map[*regexp.Regexp]int))
	dirName := "C:\\doc\\Projects\\pastex2\\pkg\\lang_detector\\rules"
	files, err := ioutil.ReadDir(dirName)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		fmt.Println(f.Name())
		loadFile(path.Join(dirName, f.Name()))
	}
}

func loadFile(fileName string) {
	langRules := make(map[*regexp.Regexp]int)
	file, err := os.Open(fileName)
	langName := path_util.GetFileNameWithoutExt(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// ignore commment
		if strings.HasPrefix(line, "//") {
			continue
		}
		tmp := strings.SplitN(line, "\t", 2)
		if len(tmp) != 2 {
			continue
		}
		reg, err := regexp.Compile(tmp[1])
		if err != nil {
			log.Fatalf("%s, regex: %s", err.Error(), tmp[1])
		}
		points, err := strconv.Atoi(tmp[0])
		if err != nil {
			log.Fatal(err)
		}
		langRules[reg] = points
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	rules[langName] = langRules
	log.Printf("loaded: %s, total: %d", langName, len(rules))
}

func createRank() *map[string]int {
	rank := make(map[string]int)
	for lang := range rules {
		rank[lang] = 0
	}
	return &rank
}

func Detect(code string) string {
	rank := *createRank()
	for langName, langRules := range rules {
		for langRule, points := range langRules {
			rank[langName] += points * len(langRule.FindAllString(code, -1))
		}
	}
	return getWinner(&rank, 0)
}

func getWinner(rank *map[string]int, thresh int) string {
	max := thresh
	winner := ""
	for name, points := range *rank {
		log.Printf("name: %s, points: %d", name, points)
		if points > max {
			max = points
			winner = name
		}
	}
	return winner
}
