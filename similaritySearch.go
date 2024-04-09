package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Histo struct {
	Name string
	H    []int
}

func computeHistogram(imagePath string, depth int) (Histo, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return Histo{"", nil}, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return Histo{"", nil}, err
	}

	bounds := img.Bounds()
	width, height := bounds.Max.X, bounds.Max.Y

	// Initialize histogram bins for each color channel
	histogram := make([]int, depth+1)

	// Iterate over each pixel in the image
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Get the color of the current pixel
			color := img.At(x, y)

			// Convert the color to RGBA
			red, green, blue, _ := color.RGBA()

			// Scale the color intensities to the desired depth
			red >>= 8   //(16-unint(depth))
			green >>= 8 //(16-unint(depth))
			blue >>= 8  //(16-unint(depth))

			// Debugging statements to inspect the values of redIndex, greenIndex, and blueIndex
			//fmt.Printf("Pixel at (%d, %d): R=%d, G=%d, B=%d\n", x, y, red, green, blue)

			// Compute the index of the histogram bin for each channel
			redIndex := int(red)
			greenIndex := int(green)
			blueIndex := int(blue)

			// Debugging statements to inspect the values of redIndex, greenIndex, and blueIndex
			//fmt.Printf("Index for pixel at (%d, %d): R=%d, G=%d, B=%d\n", x, y, redIndex, greenIndex, blueIndex)

			// Increment the corresponding bin in the histogram
			histogram[redIndex]++
			histogram[greenIndex]++
			histogram[blueIndex]++
		}
	}

	// Display RGB values for the first 5x5 pixels
	// remove y < 5 and x < 5  to scan the entire image
	/*for y := 0; y < height && y < 5; y++ {
		for x := 0; x < width && x < 5; x++ {

			// Convert the pixel to RGBA
			red, green, blue, _ := img.At(x, y).RGBA()
			// A color's RGBA method returns values in the range [0, 65535].
			// Shifting by 8 reduces this to the range [0, 255].
			red >>= 8
			blue >>= 8
			green >>= 8

			// Display the RGB values
			//fmt.Printf("Pixel at (%d, %d): R=%d, G=%d, B=%d\n", x, y, red, green, blue)
		}
	} */

	// Create a Histo struct to store the histogram
	h := Histo{imagePath, histogram}

	return h, nil
}

func computeHistograms(imagePaths []string, depth int, hChan chan<- Histo) {
	for _, imagePath := range imagePaths {
		histogram, err := computeHistogram(imagePath, depth)
		if err != nil {
			fmt.Printf("Error computing histogram for %s: %v\n", imagePath, err)
			continue
		}
		hChan <- histogram
	}
}

// Function to calculate intersection distance between two histograms
func calculateIntersectionDistance(hist1, hist2 []int) int {
	// Ensure both histograms have the same length
	if len(hist1) != len(hist2) {
		return -1
	}

	// Calculate intersection distance
	distance := 0
	for i := range hist1 {
		distance += min(hist1[i], hist2[i])
	}

	return distance
}

// Function to find the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: go run similaritySearch <queryImageFilenameDirectory_!!> <imageDatasetDirectory>")
		return
	}

	queryImageFilename := os.Args[1]
	imageDatasetDirectory := os.Args[2]
	depth := 255 // Set the depth for the histogram

	var wg sync.WaitGroup

	histogramChannel := make(chan Histo)

	//reads the contents of the image dataset directory to obtain a list of image files
	files, err := ioutil.ReadDir(imageDatasetDirectory)
	if err != nil {
		log.Fatal(err)
	}

	//filters the list of files to only include those with a .jpg extension
	var jpgFiles []string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".jpg") {
			jpgFiles = append(jpgFiles, filepath.Join(imageDatasetDirectory, file.Name()))
		}
	}

	// Insert debugging statement to print the list of JPG files
	//fmt.Println("JPG Files in Dataset:", jpgFiles)

	numThreads := 5
	batchSize := (len(jpgFiles) + numThreads - 1) / numThreads

	for i := 0; i < len(jpgFiles); i += batchSize {
		end := i + batchSize
		if end > len(jpgFiles) {
			end = len(jpgFiles)
		}
		wg.Add(1)
		go func(paths []string) {
			computeHistograms(paths, depth, histogramChannel)
			wg.Done()
		}(jpgFiles[i:end])
	}

	wg.Add(1)
	go func() {
		queryHistogram, err := computeHistogram(queryImageFilename, depth)
		fmt.Println("Histogram of the query image:", queryHistogram.H)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		histogramChannel <- queryHistogram
		wg.Done()
	}()

	var similarImages []Histo
	queryHistogram := <-histogramChannel
	for i := 0; i < len(jpgFiles); i++ {
		histogram := <-histogramChannel

		// Calculate intersection distance between query histogram and current histogram
		distance := calculateIntersectionDistance(queryHistogram.H, histogram.H)

		// Add current image to similar images list if it's one of the top 5 most similar
		if len(similarImages) < 5 {
			similarImages = append(similarImages, histogram)
		} else {
			// Replace the image with the smallest intersection distance in the list
			minDistanceIdx := 0
			for j := 1; j < 5; j++ {
				if calculateIntersectionDistance(queryHistogram.H, histogram.H) < calculateIntersectionDistance(queryHistogram.H, similarImages[minDistanceIdx].H) {
					minDistanceIdx = j
				}
			}
			// Replace image with smaller distance
			if distance > calculateIntersectionDistance(queryHistogram.H, similarImages[minDistanceIdx].H) {
				similarImages[minDistanceIdx] = histogram
			}
		}
	}
	// Wait for all goroutines to finish
	wg.Wait()

	// Close the histogramChannel after all goroutines finish sending histograms
	close(histogramChannel)

	fmt.Println("The 5 most similar images:")
	for i, hist := range similarImages {
		fmt.Printf("%d: %s\n", i+1, hist.Name)
	}

}
