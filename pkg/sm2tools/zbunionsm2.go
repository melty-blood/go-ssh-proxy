package sm2tools

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/deatil/go-cryptobin/cryptobin/sm2"
)

func TempNewSm2() {

	data := "C18615FEEB"

	pubStr := "048451CE0D8C6B4B314A08ACCD7D9A9ECC2DA44574D963D2BD2CBD1785AA1782B90D659D94FC0C196F70E316B7672DE91A6AE32E603AE758CBFE551CCBD517B40E"
	priStr := "00A4970E603CD29680296CA61B76A909B1B6082E5E06AA3577D64B0035C2A2656A"

	// {\\"cate_live\\":\\"LoveLive TypeMoon\\",\\"name\\":\\"Melty-Blood\\"}"},"message":"success"}
	// request := "999E6020A7E1846B0E8C413419F348454C29FFFC38DFFF40CDA462F8F1F145D36EA9A411DC4AE4E2CEE863C99DAD2E22525D56F59CA7CD9095D23DAFCBF65B63CC80DB21795ACBB24FB6576867881AA32C101BE512E01179B8DF23F5C2FE2D9BE2A548E0435C697AA5D11C5702911088AE5EF81B6436E0D434EA6AD50FC4FE04487C0451708945B121136CF8FA74D585082C0C0F72AF6BEB226D23FDC3B6954F02AB7FAD4C008543356DF259D7EBF4DD49C2CA61C432D464ACF9BCC0120C3131DEADC96AC7113D039E11EA66F3B59E97B18B042982B9931776C66CE9FEBFA9A8"
	// aeskey := "BA+RKN7euE774MV1FAfag1Ns+E1GLmVDDfktTgDjWWPzQRudSMqtJVxWG3i/kg7HCC5XBHc3IUtODt1EwDYhwP1tkzNg003W3ZasQDfc5cCrMzwMfWwG5j2MZgTrKci7EGAnqzK1YHjnFpnhkkM42rpZeBBc"
	// iv := "BK0shRJaKxi1TLO7JydAiDuzWdZo1KEZea0j1gP8ZSbXdpK3SIWj+fZ+CvQgnuUAQfozeEjacsZ6Zwjv05DFYwLjNAfWxpmzGJsrXG/BcGTf16+MjSyNCL6RiDyjQ67XLwlGDPe/Aux/JH7v3ql1qWPfkPx1"

	fmt.Println(12312321)
	// pub, err := hex.DecodeString(pubStr)
	pri, err := hex.DecodeString(priStr)
	if err != nil {
		fmt.Println("DecodeString, err: ", err)
	}
	fmt.Println(pri)

	if true {
		return
	}

	// 私钥
	priBytes, priBytesErr := base64.StdEncoding.DecodeString(priStr)
	// 公钥
	pubBytes, pubBytesErr := base64.StdEncoding.DecodeString(pubStr)

	dataBytes, dataBytesErr := base64.StdEncoding.DecodeString(data)
	if priBytesErr != nil || pubBytesErr != nil || dataBytesErr != nil {
		fmt.Println("DecodeString, err: private key base64 decode file ", priBytesErr, pubBytesErr, dataBytesErr)
	}

	fmt.Println(pubBytes, dataBytes)

	var result string = ""
	// 解密
	result = sm2.FromBase64String(data).FromPKCS8PrivateKeyDer(priBytes).SetMode("C1C2C3").Decrypt().ToString()

	// 验签
	// verifySign := sm2.FromBase64String(sign).FromPublicKeyDer(pubBytes).VerifyASN1(dataBytes).ToVerify()
	// fmt.Println("sm2tools->Sm2Decode", result, verifySign)
	// if !verifySign {
	// 	return "", errors.New("sign verify fail")
	// }

	fmt.Println(result)
}
