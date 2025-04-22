package acgpic

import (
	"fmt"
	"honoka/pkg/helpers"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// 灰度化并缩放图片
func resizeAndGrayscale(img image.Image, width, height int) []float64 {
	result := make([]float64, width*height)
	bounds := img.Bounds()
	scaleX := float64(bounds.Dx()) / float64(width)
	scaleY := float64(bounds.Dy()) / float64(height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := int(float64(x) * scaleX)
			srcY := int(float64(y) * scaleY)
			gray := color.GrayModel.Convert(img.At(srcX, srcY)).(color.Gray)
			result[y*width+x] = float64(gray.Y)
		}
	}
	return result
}

// 计算离散余弦变换 (DCT)
func dctTransform(data []float64, size int) []float64 {
	dct := make([]float64, len(data))
	for u := 0; u < size; u++ {
		for v := 0; v < size; v++ {
			var sum float64
			for x := 0; x < size; x++ {
				for y := 0; y < size; y++ {
					cos1 := math.Cos((2*float64(x) + 1) * float64(u) * math.Pi / (2 * float64(size)))
					cos2 := math.Cos((2*float64(y) + 1) * float64(v) * math.Pi / (2 * float64(size)))
					sum += data[y*size+x] * cos1 * cos2
				}
			}
			cu := 1.0
			if u == 0 {
				cu = math.Sqrt(0.5)
			}
			cv := 1.0
			if v == 0 {
				cv = math.Sqrt(0.5)
			}
			dct[v*size+u] = sum * cu * cv / 4
		}
	}
	return dct
}

// 生成感知哈希
func generatePHash(img image.Image) uint64 {
	const size = 8
	const dctSize = 32

	// 步骤 1: 灰度化和缩放
	data := resizeAndGrayscale(img, dctSize, dctSize)

	// 步骤 2: 计算 DCT
	dct := dctTransform(data, dctSize)

	// 步骤 3: 取前 8x8 的 DCT 系数（去掉直流分量）
	var avg float64
	for i := 1; i < size*size; i++ {
		avg += dct[i]
	}
	avg /= float64(size*size - 1)

	// 步骤 4: 生成哈希
	var hash uint64
	for i := 1; i < size*size; i++ {
		if dct[i] > avg {
			hash |= 1 << (i - 1)
		}
	}
	return hash
}

// 计算汉明距离
func hammingDistance(hash1, hash2 uint64) int {
	xor := hash1 ^ hash2
	distance := 0
	for xor > 0 {
		distance += int(xor & 1)
		xor >>= 1
	}
	return distance
}

// 遍历目录，查找相似图片
func findSimilarImages(targetPath, searchDir string, threshold int) ([]string, error) {
	resultArr := []string{}
	fileExtMap := map[string]int{
		"jpg":  1,
		"png":  1,
		"jpeg": 1,
	}
	file, err := os.Open(targetPath)
	if err != nil {
		fmt.Println("无法打开目标图片:", err)
		return resultArr, err
	}
	defer file.Close()
	fmt.Println(targetPath, err, file)

	img, _, err := image.Decode(file)
	if err != nil {
		fmt.Println("无法解码目标图片:", err)
		return resultArr, err
	}

	targetHash := generatePHash(img)

	searchChan := make(chan string, 100)
	var lockRW sync.RWMutex
	// 计算文件数量(不包括目录)
	fileDirCount := 0
	err = filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		tempExt := strings.ToLower(helpers.FileExtNoPoint(info.Name()))
		if _, ok := fileExtMap[tempExt]; !ok {
			return nil
		}
		fileDirCount++
		return nil
	})
	if err != nil {
		fmt.Println("Error get filedir count err: ", err)
		return resultArr, err
	}
	fmt.Println("filedir :", searchDir, " | count: ", fileDirCount)

	scanFileCount := 0
	err = filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		tempExt := strings.ToLower(helpers.FileExtNoPoint(info.Name()))
		if _, ok := fileExtMap[tempExt]; !ok {
			fmt.Println("Error this picture is not jpg or png ", info.Name())
			return nil
		}
		if !info.IsDir() {

			go func() {
				defer func() {
					lockRW.Lock()
					scanFileCount++
					lockRW.Unlock()
				}()
				searchFile, err := os.Open(path)
				if err != nil {
					return
				}
				defer searchFile.Close()

				searchImg, _, err := image.Decode(searchFile)
				if err != nil {
					return
				}

				searchHash := generatePHash(searchImg)
				distance := hammingDistance(targetHash, searchHash)
				// fmt.Println("go func on fileWalk: ", info.Name(), scanFileCount, " | hamming: ", distance)
				if distance <= threshold {
					fmt.Printf("找到相似图片: %s (汉明距离: %d)\n", path, distance)
					searchChan <- path
					return
				}
			}()
		}

		return nil
	})
	if err != nil {
		fmt.Println("filepath.Walk: ", err)
		return resultArr, err
	}
	for {
		select {
		case pathChan, ok := <-searchChan:
			resultArr = append(resultArr, pathChan)
			fmt.Println("for select case1 ", pathChan, ok)
		default:
			fmt.Println("for select default: default wait")
			time.Sleep(time.Second)
			if scanFileCount >= fileDirCount {
				fmt.Println("for select default: scan over ", scanFileCount, fileDirCount)
				return resultArr, nil
			}
		}
	}

}

func SearchPic(targetImg, searchImgDir string, threshold int) {
	runtime.GOMAXPROCS(60)

	// opt := SearchPicOpt{IsNeedWritePic: false}
	// result, _ := SearchPicCUDA(targetImg, searchImgDir, threshold, &opt)

	result, _ := findSimilarImages(targetImg, searchImgDir, threshold)
	fmt.Println("result: ", result)
	for _, val := range result {
		fmt.Println("image path: ", val)
	}
}

// gocv有问题导致无法打包, 后续在做修复
func SearchPicCUDA(targetPath, searchDir string, threshold int, opt *SearchPicOpt) ([]string, error) {
	resultArr := []string{}
	// fileExtMap := map[string]int{
	// 	"jpg":  1,
	// 	"png":  1,
	// 	"jpeg": 1,
	// }

	// // 加载目标图片
	// targetImg := gocv.IMRead(targetPath, gocv.IMReadGrayScale)
	// if targetImg.Empty() {
	// 	fmt.Println("无法加载目标图片")
	// 	return resultArr, errors.New("can not load target filepic")
	// }
	// defer targetImg.Close()

	// // 创建 ORB 特征检测器
	// orb := gocv.NewORB()
	// defer orb.Close()

	// // 检测目标图片的特征点和描述符
	// targetKeyPoints, targetDescriptors := orb.DetectAndCompute(targetImg, gocv.NewMat())

	// matcher := gocv.NewBFMatcher()
	// defer matcher.Close()

	// scanFileCount := 0
	// fileDirCount, _ := helpers.DirFilesCount(searchDir)
	// fmt.Println("filedir :", searchDir, " | count: ", fileDirCount, scanFileCount)
	// searchChan := make(chan string, 210)

	// err := filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
	// 	if info.IsDir() {
	// 		return nil
	// 	}
	// 	tempExt := strings.ToLower(helpers.FileExtNoPoint(info.Name()))
	// 	if _, ok := fileExtMap[tempExt]; !ok {
	// 		fmt.Println("此图片格式不是jpg或png ", info.Name())
	// 		return nil
	// 	}

	// 	// 加载搜索图片
	// 	searchImg := gocv.IMRead(path, gocv.IMReadGrayScale)
	// 	if searchImg.Empty() {
	// 		fmt.Printf("无法加载搜索目录下图片: %s\n", info.Name())
	// 		// return errors.New("can not load searchDir picture" + info.Name())
	// 	}
	// 	defer searchImg.Close()

	// 	// 检测搜索图片的特征点和描述符
	// 	searchKeyPoints, searchDescriptors := orb.DetectAndCompute(searchImg, gocv.NewMat())

	// 	// 进行特征点匹配
	// 	matches := matcher.KnnMatch(targetDescriptors, searchDescriptors, 2)
	// 	goodMatches := []gocv.DMatch{}
	// 	for _, m := range matches {
	// 		if len(m) == 2 && m[0].Distance < 0.75*m[1].Distance {
	// 			goodMatches = append(goodMatches, m[0])
	// 		}
	// 	}

	// 	// 如果良好匹配数大于一定阈值，认为找到匹配图片
	// 	if len(goodMatches) > threshold {
	// 		fmt.Printf("找到匹配图片: %s (良好匹配数: %d)\n", info.Name(), len(goodMatches))
	// 		searchChan <- path
	// 		resultArr = append(resultArr, path)
	// 		if opt.IsNeedWritePic {
	// 			_, hasErr := os.ReadDir(opt.WriteResultPicPath)
	// 			if hasErr != nil {
	// 				fmt.Println("Error write result picturee fail: dir not exists: ", opt.WriteResultPicPath)
	// 				return hasErr
	// 			}

	// 			matchColor := color.RGBA{166, 222, 0, 0}
	// 			singlePointColor := color.RGBA{255, 255, 0, 0}
	// 			// gocv.NewScalar(0, 255, 0, 0), gocv.NewScalar(255, 0, 0, 0)
	// 			// 保存匹配结果图片
	// 			result := gocv.NewMat()
	// 			defer result.Close()
	// 			gocv.DrawMatches(targetImg, targetKeyPoints, searchImg, searchKeyPoints, goodMatches, &result, matchColor, singlePointColor, nil, gocv.DrawDefault)

	// 			outputPath := fmt.Sprintf("/result_%s", info.Name())
	// 			outPutPic := opt.WriteResultPicPath + outputPath
	// 			gocv.IMWrite(outPutPic, result)
	// 			fmt.Printf("匹配结果保存到: %s\n", outputPath)
	// 		}
	// 	} else {
	// 		fmt.Printf("图片 %s 未找到足够的匹配 (匹配数: %d)\n", info.Name(), len(goodMatches))
	// 	}
	// 	return nil
	// })

	// if err != nil {
	// 	fmt.Println("filepath.Walk: ", err)
	// 	return resultArr, err
	// }
	// for {
	// 	select {
	// 	case pathChan, ok := <-searchChan:
	// 		resultArr = append(resultArr, pathChan)
	// 		fmt.Println("for select case1 ", pathChan, ok)
	// 	default:
	// 		fmt.Println("for select default: default wait")
	// 		time.Sleep(time.Second)
	// 		if scanFileCount >= fileDirCount {
	// 			fmt.Println("for select default: scan over", scanFileCount, fileDirCount)
	// 			return resultArr, nil
	// 		}
	// 	}
	// }
	return resultArr, nil
}

type SearchPicOpt struct {
	IsNeedWritePic     bool
	WriteResultPicPath string
}
