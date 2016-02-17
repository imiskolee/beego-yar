package beego_yar
import (
	"github.com/astaxie/beego/context"
	"errors"
	"bytes"
	"beego-yar/packager"
	"github.com/astaxie/beego"
	"reflect"
)

type Server struct {

	ctx *context.Context
	class interface{}
	body []byte
}


func NewServer(ctx *context.Context,class interface{}) *Server {
	server := new(Server)
	server.class = class
	server.ctx = ctx
	return server
}


func (self *Server)getHeader() (*Header,error) {

	header_buffer := bytes.NewBuffer(self.body[0:PROTOCOL_LENGTH+PROTOCOL_LENGTH])

	header := NewHeaderWithBytes(header_buffer)

	if header.MagicNumber != MAGIC_NUMBER {

		return nil,errors.New("magic number check failed.")

	}

	return header,nil

}



func (self *Server)getRequest(header *Header) (*Request,error) {

	body_len := header.BodyLength

	body_buffer := self.body[90:90+body_len-8]

	request := NewRequest()

	beego.Debug(body_len)

	err := packager.Unpack(header.Packager[:],body_buffer,request)

	if err != nil {

		return nil,err

	}

	return request,nil
}


func (self *Server)sendResponse(response *Response) error {

	sendPackData,err := packager.Pack(response.Protocol.Packager[:],response)

	if err != nil {

		return err
	}
	response.Protocol.BodyLength = uint32(len(sendPackData) +8)
	self.ctx.ResponseWriter.Write(response.Protocol.Bytes().Bytes())
	self.ctx.ResponseWriter.Write(sendPackData)
	return nil

}

func (self *Server)call(request *Request,response *Response) {

	call_params := request.Params.([]interface{})

	class_fv := reflect.ValueOf(self.class)

	_,err := class_fv.Type().MethodByName(request.Method)

	if err == false {
		response.Status = ERR_EMPTY_RESPONSE
		response.Error = "call undefined api:" + request.Method
		return
	}

	fv :=class_fv.MethodByName(request.Method)

	if len(call_params) != fv.Type().NumIn() {

		response.Status = ERR_EMPTY_RESPONSE
		response.Error = "mismatch call param nums"
		return
	}

	real_params := make([]reflect.Value, len(call_params))
	real_params[0] = class_fv

	func() {

		for i, v := range call_params {
			raw_val := reflect.ValueOf(v)
			real_params[i] = raw_val.Convert(fv.Type().In(i))
		}

		rs := fv.Call(real_params)
		if len(rs) < 1 {
			response.Return(nil)
		}

		if len(rs) > 1 {
			response.Status = ERR_EMPTY_RESPONSE
			response.Error = "unsupprted multi value return on rpc call"
			return
		}

		response.Return(rs[0].Interface())
	}()

}

func (self *Server)Handle() (bool,error){

	self.body = self.ctx.Input.RequestBody

	var err error

	if len(self.body) < (PROTOCOL_LENGTH + PACKAGER_LENGTH) {

		return false,errors.New("read request body error.")
	}

	header,err := self.getHeader()

	if err != nil {
		beego.Error(err)
		return false,err
	}

	request,err := self.getRequest(header)

	if err != nil {
		beego.Error(err)
		return false,err
	}

	response := NewResponse()
	response.Status = ERR_OKEY
	response.Protocol = header
	self.call(request,response)
	self.sendResponse(response)

	if response.Status != ERR_OKEY {

		beego.Warn(request.Id,request.Method,response.Error)

	}else {

		beego.Debug(request.Id,request.Method,"OKEY")

	}


	return true,nil
}