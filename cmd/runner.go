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
	"time"
)

func getDBDirname() string {
	if len(os.Args) > 1 {
		return os.Args[1]
	} else {
		return "/Users/wyatttall/git/spotifaux/The Beatles"
	}
}

func main() {
	sourceWavFileName := "/Users/wyatttall/git/spotifaux/recreate/kick.wav"

	e := spotifaux.NewFeatureExtractor(spotifaux.SAMPLE_RATE)
	s := &spotifaux.SoundSpotter{
		ChosenFeatures: []int{3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		CqtN:           e.CqtN,
		ShingleSize:    11,
	}

	//dbMp3sToWavs()
	dbWavsToDats(e)

	err := e.ExtractSeriesOfVectors(sourceWavFileName, toDat(sourceWavFileName))
	if err != nil {
		panic(err)
	}

	sourceDatToRecipe(toDat(sourceWavFileName), s)
	recipeToOutput(sourceWavFileName, s)
}

func dbMp3sToWavs() {
	dirname := getDBDirname()
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

func dbWavsToDats(e *spotifaux.FeatureExtractor) {

	dirname := getDBDirname()
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

func sourceDatToRecipe(sourceDatFileName string, s *spotifaux.SoundSpotter) {

	source, err := spotifaux.NewDatReader(sourceDatFileName, s.CqtN)
	if err != nil {
		panic(err)
	}

	x := source.Frames
	if x%s.ShingleSize > 0 {
		x += s.ShingleSize - source.Frames%s.ShingleSize
	}
	s.InShingles = make([][]float64, x)

	for d := 0; d < x; d++ {
		s.InShingles[d], err = source.Dat()
		if err == io.EOF {
			s.InShingles[d] = make([]float64, s.CqtN)
		} else if err != nil {
			panic(err)
		}
	}

	err = source.Close()
	if err != nil {
		panic(err)
	}

	dirname := getDBDirname()
	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		panic(err)
	}

	x = len(s.InShingles) / s.ShingleSize
	if len(s.InShingles)%s.ShingleSize > 0 {
		x++
	}

	totalTime := time.Duration(0)
	wavCount := 0

	winners := make([]spotifaux.Winner, x)
	for i, fi := range files {
		if strings.HasSuffix(fi.Name(), ".wav") {

			start := time.Now()
			wavFileName := dirname + "/" + fi.Name()
			fileWinners, err := spotifaux.Match(wavFileName, toDat(wavFileName), s)
			if err != nil {
				panic(err)
			}

			subs := 0
			for w := 0; w < len(winners); w++ {
				if (winners[w] == spotifaux.Winner{}) || fileWinners[w].MinDist < winners[w].MinDist {
					winners[w] = fileWinners[w]
					subs++
				}
			}

			writeRecipe(winners) // overwrite recipe each time

			totalTime += time.Since(start)
			wavCount++
			estDone := time.Now().Add(totalTime / time.Duration(wavCount) * time.Duration((len(files)-i)/3))

			fmt.Printf("  %d of %d %s subs %d/%d estDone %s\n", i, len(files), fi.Name(), subs, x,
				estDone.Format("15:04:05"))
		}
	}
}

func writeRecipe(winners []spotifaux.Winner) {
	recipe, err := os.Create("recipe.json")
	if err != nil {
		panic(err)
	}
	defer recipe.Close()

	_, err = recipe.WriteString("{\"recipe\":[\n")
	if err != nil {
		panic(err)
	}

	for w, winner := range winners {
		maybeComma := ","
		if w == len(winners)-1 {
			maybeComma = ""
		}

		w := fmt.Sprintf("{\"file\":\"%s\",\"winner\":%d}%s\n", winner.File, winner.Winner, maybeComma)
		_, err = recipe.WriteString(w)
		if err != nil {
			panic(err)
		}
	}

	_, err = recipe.WriteString("]}")
	if err != nil {
		panic(err)
	}
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

		output, err := s.Output(winner.File, winner.Winner, inPower)
		if err != nil {
			panic(err)
		}

		wavWriter.WriteItems(output)
	}
}

func readRecipe() spotifaux.Recipe {
	recipeFile, err := os.Open("recipe.json")
	if err != nil {
		fmt.Println(err)
	}
	defer recipeFile.Close()

	byteValues, err := ioutil.ReadAll(recipeFile)
	if err != nil {
		panic(err)
	}

	var recipe spotifaux.Recipe
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
