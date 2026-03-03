package main

// All CSS selectors for the ICP booking site, centralized for easy updates.
const (
	// Page 1: Oficina + Tramite selection
	SelOfficeInitial = "#sede"
	SelTramiteSelect = "#tramiteGrupo\\[0\\]"
	SelBtnAceptar    = "#btnAceptar"

	// Page 2: Info page with Entrar button
	SelBtnEntrar = "#btnEntrar"

	// Page 3: Personal data form
	SelCountry     = "#txtPaisNac"
	SelDocNIE      = "#rdbTipoDocNie"
	SelDocPassport = "#rdbTipoDocPas"
	SelDocDNI      = "#rdbTipoDocDni"
	SelDocNumber   = "#txtIdCitado"
	SelName        = "#txtDesCitado"
	SelBtnEnviar   = "#btnEnviar"

	// Page 4: Office selection (may appear after personal data)
	SelOfficeLater  = "#idSede"
	SelBtnSiguiente = "#btnSiguiente"

	// Page 5: Contact info
	SelPhone  = "#txtTelefonoCitado"
	SelEmail1 = "#emailUNO"
	SelEmail2 = "#emailDOS"
)
