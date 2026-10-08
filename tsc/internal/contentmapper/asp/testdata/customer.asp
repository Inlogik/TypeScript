<%
/**
 * @typedef {Object} Customer
 * @property {number} Id
 * @property {string} Name
 */
/** @param {number} id @returns {Customer} */
function GetCustomer(id) {
    return { Id: id, Name: "Example" };
}
%>
