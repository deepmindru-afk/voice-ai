// Copyright (c) 2023-2025 RapidaAI
// Author: Prashant Srivastav <prashant@rapida.ai>
//
// Licensed under GPL-2.0 with Rapida Additional Terms.
// See LICENSE.md or contact sales@rapida.ai for commercial usage.
package internal_assistant_entity

type AssistantAuthentication struct {
	AssistantId uint64 `json:"assistantId" gorm:"type:bigint;size:20"`
	// for now we are only supporting single authentication per assistant, in future we can support multiple authentication with different providers
	AuthenticationProvider   string `json:"authenticationProvider" gorm:"type:string;size:50;not null;default:OAUTH2"`
	AuthenticationProviderId uint64 `json:"authenticationProviderId" gorm:"type:bigint;size:20"`
}
