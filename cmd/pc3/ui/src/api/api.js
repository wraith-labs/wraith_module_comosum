export default class API {

    getCustomersSmall() {
        return fetch('data/customers-small.json').then(res => res.json()).then(d => d.data);
    }

}
  