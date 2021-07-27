/*
 utility methods

*/

function ajaxJsonRequest(url, method, data, successfn, errorfn) {
	dataContent = ''
	if (data !== '') {
		dataContent = data
		
	}

	$.ajax({
		type: method,
		dataType: "json",
		contentType: "application/json",
		url: url,
		data: dataContent,
		success: successfn,
		error: errorfn
	});
}


