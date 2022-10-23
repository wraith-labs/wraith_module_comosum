<template>
	<div class="grid">
		<div class="col-12">
			<div class="card">
				<h5>About Page</h5>
				<p>Use this page to start from scratch and place your custom content.</p>
			</div>
		</div>
	</div>
</template>

<script>
import API from '../../api/api';

export default {
	data() {
		return {
			products: null,
			productDialog: false,
			deleteProductDialog: false,
			deleteProductsDialog: false,
			product: {},
			selectedProducts: null,
			filters: {},
			submitted: false,
			statuses: [
				{label: 'INSTOCK', value: 'instock'},
				{label: 'LOWSTOCK', value: 'lowstock'},
				{label: 'OUTOFSTOCK', value: 'outofstock'}
			]
		}
	},
	productService: null,
	created() {
		this.service = new API();
		this.initFilters();
	},
	mounted() {
		this.productService.getProducts().then(data => this.products = data);
	},
	methods: {
		formatCurrency(value) {
			if(value)
				return value.toLocaleString('en-US', {style: 'currency', currency: 'USD'});
			return;
		},
	}
}
</script>