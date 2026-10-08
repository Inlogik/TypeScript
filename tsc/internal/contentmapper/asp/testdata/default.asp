<%@ Language=JavaScript %>
<!-- #include file="customer.asp" -->
<!-- #include virtual="/helpers.asp" -->
<%
var id = Request.QueryString("id");
var customer = GetCustomer(id);
Response.Write(customer.Name);
%>
<h1><%= customer.DoesNotExist %></h1>
