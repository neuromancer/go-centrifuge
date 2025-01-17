// +build testworld

package testworld

func defaultDocumentPayload(documentType string, collaborators []string) map[string]interface{} {

	switch documentType {
	case typeInvoice:
		return defaultInvoicePayload(collaborators)
	case typePO:
		return defaultPOPayload(collaborators)
	default:
		return defaultInvoicePayload(collaborators)
	}

}

func defaultPOPayload(collaborators []string) map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"po_number":    "12324",
			"due_date":     "2018-09-26T23:12:37.902198664Z",
			"gross_amount": "40",
			"currency":     "USD",
			"net_amount":   "40",
		},
		"collaborators": collaborators,
	}

}
func defaultInvoicePayload(collaborators []string) map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"invoice_number": "12324",
			"due_date":       "2018-09-26T23:12:37.902198664Z",
			"gross_amount":   "40",
			"currency":       "USD",
			"net_amount":     "40",
		},
		"collaborators": collaborators,
	}

}

func invoiceNFTPayload(collaborators []string) map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"invoice_number": "12324",
			"due_date":       "2018-09-26T23:12:37.902198664Z",
			"gross_amount":   "40",
			"currency":       "USD",
			"net_amount":     "40",
			"document_type":  "invoice",
			"sender":         collaborators[0],
			"invoice_status": "unpaid",
		},
		"collaborators": collaborators,
	}

}

func poNFTPayload(collaborators []string) map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"po_number":     "123245",
			"due_date":      "2018-09-26T23:12:37.902198664Z",
			"currency":      "USD",
			"net_amount":    "40",
			"document_type": "po",
		},
		"collaborators": collaborators,
	}

}

func defaultNFTPayload(documentType string, collaborators []string) map[string]interface{} {

	switch documentType {
	case typeInvoice:
		return invoiceNFTPayload(collaborators)
	case typePO:
		return poNFTPayload(collaborators)
	default:
		return invoiceNFTPayload(collaborators)
	}

}

func updatedDocumentPayload(documentType string, collaborators []string) map[string]interface{} {
	switch documentType {
	case typeInvoice:
		return updatedInvoicePayload(collaborators)
	case typePO:
		return updatedPOPayload(collaborators)
	default:
		return updatedInvoicePayload(collaborators)
	}
}

func updatedPOPayload(collaborators []string) map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"po_number":  "12324",
			"due_date":   "2018-09-26T23:12:37.902198664Z",
			"currency":   "EUR",
			"net_amount": "42",
		},
		"collaborators": collaborators,
	}

}

func updatedInvoicePayload(collaborators []string) map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"invoice_number": "12324",
			"due_date":       "2018-09-26T23:12:37.902198664Z",
			"gross_amount":   "40",
			"currency":       "EUR",
			"net_amount":     "42",
		},
		"collaborators": collaborators,
	}

}

func defaultProofPayload(documentType string) map[string]interface{} {
	if documentType == typeInvoice {

		return map[string]interface{}{
			"type":   "http://github.com/centrifuge/centrifuge-protobufs/invoice/#invoice.InvoiceData",
			"fields": []string{"invoice.net_amount", "invoice.currency"},
		}
	}
	return map[string]interface{}{
		"type":   "http://github.com/centrifuge/centrifuge-protobufs/purchaseorder/#purchaseorder.PurchaseOrderData",
		"fields": []string{"po.net_amount", "po.currency"},
	}
}
