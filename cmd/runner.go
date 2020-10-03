package main

import (
	"encoding/json"
	"fmt"
	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/wav"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"spotifaux"
	"strings"
)

func main() {
	var dbDirname string
	if len(os.Args) > 1 {
		dbDirname = os.Args[1]
	} else {
		dbDirname = "/Users/wyatttall/git/spotifaux/db"
	}
	sourceWavFileName := "/Users/wyatttall/git/spotifaux/recreate/all-short.wav"

	e := spotifaux.NewFeatureExtractor(spotifaux.SAMPLE_RATE)
	s := spotifaux.NewSoundSpotter(e.CqtN)

	//dbMp3sToWavs(dbDirname)
	dbWavsToDats(dbDirname, e)

	err := e.ExtractSeriesOfVectors(sourceWavFileName, toDat(sourceWavFileName))
	if err != nil {
		panic(err)
	}

	sourceToRecipe(toDat(sourceWavFileName), s, dbDirname)
	recipeToOutput(sourceWavFileName, s)
}

func dbMp3sToWavs(dirname string) {
	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		panic(err)
	}

	for i, fi := range files {
		if strings.HasSuffix(fi.Name(), ".mp3") {
			fmt.Printf("%d of %d %s to wav\n", i, len(files), fi.Name())

			f, err := os.Open(dirname + "/" + fi.Name())
			if err != nil {
				panic(err)
			}

			streamer, _, err := mp3.Decode(f)
			if err != nil {
				log.Fatal(err)
			}

			monoStreamer := effects.Mono(streamer)

			out, err := os.Create(dirname + "/" + fi.Name()[0:strings.LastIndex(fi.Name(), ".")] + ".wav")
			if err != nil {
				panic(err)
			}

			r := beep.Resample(6, beep.SampleRate(48000), beep.SampleRate(16000), monoStreamer)

			err = wav.Encode(out, r, beep.Format{
				SampleRate:  beep.SampleRate(16000),
				NumChannels: 1,
				Precision:   1,
			})
			if err != nil {
				panic(err)
			}

			streamer.Close()
			f.Close()
			out.Close()
		}
	}
}

func dbWavsToDats(dirname string, e *spotifaux.FeatureExtractor) {

	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		panic(err)
	}

	for i, fi := range files {
		if strings.HasSuffix(fi.Name(), ".wav") {
			fmt.Printf("%d of %d %s to dat\n", i, len(files), fi.Name())

			err = e.ExtractSeriesOfVectors(dirname+"/"+fi.Name(), toDat(dirname+"/"+fi.Name()))
			if err != nil {
				panic(err)
			}
		}
	}
}

func toDat(fileName string) string {
	return fileName[0:strings.LastIndex(fileName, ".")] + ".dat"
}

func sourceToRecipe(sourceDatFileName string, s *spotifaux.SoundSpotter, dirName string) {

	recipe, err := os.Create("recipe.json")
	if err != nil {
		panic(err)
	}

	_, err = recipe.WriteString("{\"recipe\":[\n")
	if err != nil {
		panic(err)
	}

	source, err := spotifaux.NewDatReader(sourceDatFileName, s.CqtN)
	if err != nil {
		panic(err)
	}

	last := false
	for iter := 0; ; iter++ {
		for muxi := 0; muxi < s.ShingleSize; muxi++ {
			s.InShingles[muxi], err = source.Dat()
			if err == io.EOF {
				s.InShingles[muxi] = make([]float64, s.CqtN)
				last = true
			} else if err != nil {
				panic(err)
			}
		}

		// matched filter matching to get winning database shingle
		fileName, winner, err := getWinner(dirName, s)
		if err != nil {
			panic(err)
		}

		fmt.Printf("%d %s %d\n", iter, fileName[strings.LastIndex(fileName, "/")+1:], winner)

		maybeComma := ","
		if last {
			maybeComma = ""
		}

		w := fmt.Sprintf("{\"filename\":\"%s\",\"Winner\":%d}%s\n", fileName, winner, maybeComma)
		_, err = recipe.WriteString(w)
		if err != nil {
			panic(err)
		}

		if last {
			break
		}
	}

	_, err = recipe.WriteString("]}")
	if err != nil {
		panic(err)
	}
	err = recipe.Close()
	if err != nil {
		panic(err)
	}
	err = source.Close()
	if err != nil {
		panic(err)
	}
}

func getWinner(dirName string, s *spotifaux.SoundSpotter) (string, int, error) {

	files, err := ioutil.ReadDir(dirName)
	if err != nil {
		return "", 0, err
	}

	fileName := ""
	winner := -1
	minDist := 10.0
	for i, fi := range files {
		if strings.HasSuffix(fi.Name(), ".wav") {

			if i%15 == 0 {
				fmt.Printf("  %d of %d %s\n", i, len(files), fi.Name())
			}

			w, dist, err := spotifaux.Match(toDat(dirName+"/"+fi.Name()), s)
			if err != nil {
				return "", 0, err
			}

			if dist < minDist {
				fileName = dirName + "/" + fi.Name()
				winner = w
				minDist = dist
			}
		}
	}
	return fileName, winner, nil
}

type Winner struct {
	Filename string `json:"filename"`
	Winner   int    `json:"winner"`
}

type Recipe struct {
	Winner []Winner `json:"recipe"`
}

func recipeToOutput(sourceWavFileName string, s *spotifaux.SoundSpotter) {

	sf, err := spotifaux.NewSoundFile(sourceWavFileName)
	if err != nil {
		panic(err)
	}
	defer sf.Close()

	recipe := readRecipe()

	wavWriter := spotifaux.NewWavWriter("out.wav")
	defer wavWriter.Close()

	for _, winner := range recipe.Winner {

		inPower, err := getInPower(sf, spotifaux.Hop*s.ShingleSize)
		if err != nil {
			panic(err)
		}

		output, err := s.Output(winner.Filename, winner.Winner, inPower)
		if err != nil {
			panic(err)
		}
		wavWriter.WriteItems(output)
	}
}

func readRecipe() Recipe {
	recipeFile, err := os.Open("recipe.json")
	if err != nil {
		fmt.Println(err)
	}
	defer recipeFile.Close()

	byteValues, err := ioutil.ReadAll(recipeFile)
	if err != nil {
		panic(err)
	}

	var recipe Recipe
	err = json.Unmarshal(byteValues, &recipe)
	if err != nil {
		panic(err)
	}
	return recipe
}

func getInPower(sf *spotifaux.SoundFile, bufLength int) (float64, error) {

	buf := make([]float64, bufLength)
	read, err := sf.ReadFrames(buf)
	if err != nil {
		return 0, err
	}

	inPower := 0.0
	for nn := 0; nn < int(read); nn++ {
		inPower += math.Pow(buf[nn], 2)
	}
	inPower /= float64(bufLength)

	return inPower, nil
}
