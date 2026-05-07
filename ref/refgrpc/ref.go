package refgrpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"spider-server/logger"

	"golang.org/x/net/http2"
	"google.golang.org/protobuf/proto"
)

// 创建 logger 实例
// var logger = logrus.New()

type GrpcResp struct {
	Body    []byte
	Trailer http.Header
	Header  http.Header
}

//func main() {
//	reqDta := Req()
//	gr, err := GrpcInvoke(url, reqDta, "")
//	if err != nil {
//		// 使用 logger 记录错误
//		logger.Errorf("GrpcInvoke failed: %v", err)
//		return
//	}
//	logger.Info("GrpcInvoke succeeded")
//	fmt.Println(gr)
//}

func GrpcInvoke(url string, reqpb []byte, xsUid string) (*GrpcResp, error) {
	defer func() {
		if r := recover(); r != nil {
			// 使用 logger 记录 panic
			logger.Errorf("Recovered from panic: %v", r)
		}
	}()

	// 调用 clientGrpcHttp2 函数
	gr := clientGrpcHttp2(url, reqpb, xsUid)

	// 如果没有发生 panic，正常返回结果
	if gr == nil {
		// 使用 logger 记录错误
		return nil, errors.New("clientGrpcHttp2 returned nil response")
	}

	return gr, nil
}

var client = &http.Client{
	Transport: &http2.Transport{
		AllowHTTP: true, // 允许使用不加密的 HTTP 协议
		DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr) // 使用普通连接
		},
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // 视需要调整为实际证书验证
		},
	},
}

func clientGrpcHttp2(url string, reqpb []byte, xsUid string) *GrpcResp {
	// 使用非加密的 HTTP 协议访问 gRPC 服务
	// 创建 HTTP/2 支持的 Transport

	// 构造 HTTP 请求
	req, err := http.NewRequest("POST", url, bytes.NewReader(reqpb))
	if err != nil {
		// 使用 logger 记录错误
		logger.Errorf("无法构造请求: %v", err)
		return nil
	}

	// 设置 gRPC 所需的 HTTP/2 头
	req.Header.Set("Content-Type", "application/grpc")
	req.Header.Set("TE", "trailers")
	req.Header.Set("xs_uid", xsUid)
	req.Header.Set("xs_appkey", "1")
	req.Header.Set("xx-nonce", "11111111"+
		"11111111"+
		"11111111"+
		"11111111")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		// 使用 logger 记录错误
		logger.Printf("请求失败: %v", err)
		return nil
	}

	defer resp.Body.Close()
	logger.Info("发送请求成功")

	// 读取响应数据
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		// 使用 logger 记录错误
		logger.Printf("无法读取响应: %v", err)
	}

	logger.Info("读取响应成功")

	// 读取 Trailers
	logger.Info("响应 Trailers:")
	for key, values := range resp.Trailer {
		for _, value := range values {
			logger.Printf("%s: %s \n", key, value)
		}
	}

	// 读取 Headers
	logger.Info("响应 Headers:")
	for key, values := range resp.Header {
		for _, value := range values {
			logger.Printf("%s: %s \n", key, value)
		}
	}

	logger.Info("响应数据: %s", responseData)

	return &GrpcResp{responseData, resp.Trailer, resp.Header}
}

func Req() []byte {
	// 构造请求消息并序列化为 protobuf 格式
	reqMessage := &pbservice.ProdRequest{ProdId: 1}
	serializedRequest, err := proto.Marshal(reqMessage)
	if err != nil {
		// 使用 logger 记录错误
		logger.Errorf("序列化请求数据失败: %v", err)
	}

	// 构造符合 gRPC 协议的请求数据
	var requestData bytes.Buffer

	// 写入标志位（1 字节）
	requestData.WriteByte(0) // 表示未压缩

	// 写入消息长度（4 字节，使用大端序）
	msgLength := uint32(len(serializedRequest))
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, msgLength)
	requestData.Write(lengthBytes)

	// 写入实际的消息数据
	requestData.Write(serializedRequest)
	return requestData.Bytes()
}
