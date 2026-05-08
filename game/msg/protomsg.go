package msg

import "google.golang.org/protobuf/reflect/protoreflect"

// resp := &api.SignInResponse{}
//	setStringField(resp.ProtoReflect(), "token", token)
//	setStringField(resp.ProtoReflect(), "uc_token", token)
//	setStringField(resp.ProtoReflect(), "jwt_token", token)
//	setUintField(resp.ProtoReflect(), "uid", uint64(user.ID))
//	setUintField(resp.ProtoReflect(), "user_id", uint64(user.ID))

// 反射设置 value
func setStringField(message protoreflect.Message, name string, value string) {
	field := message.Descriptor().Fields().ByName(protoreflect.Name(name))
	if field == nil || field.Kind() != protoreflect.StringKind {
		return
	}
	message.Set(field, protoreflect.ValueOfString(value))
}

func setUintField(message protoreflect.Message, name string, value uint64) {
	field := message.Descriptor().Fields().ByName(protoreflect.Name(name))
	if field == nil {
		return
	}

	switch field.Kind() {
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		message.Set(field, protoreflect.ValueOfUint64(value))
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		message.Set(field, protoreflect.ValueOfUint64(value))
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		message.Set(field, protoreflect.ValueOfInt64(int64(value)))
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		message.Set(field, protoreflect.ValueOfInt64(int64(value)))
	}
}
