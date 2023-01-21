<template>
	<div class="grid">
		<div class="col-12">
			<div class="card">
				<Toast/>
				<Toolbar class="mb-4">
					<template v-slot:start>
						<div class="my-2">
							<Button label="New" icon="pi pi-plus" class="p-button-success mr-2" @click="" />
							<Button label="Delete" icon="pi pi-trash" class="p-button-danger" @click="" :disabled="!clientsSelected || !clientsSelected.length" />
						</div>
					</template>

					<template v-slot:end>
						<FileUpload mode="basic" accept="image/*" :maxFileSize="1000000" label="Import" chooseLabel="Import" class="mr-2 inline-block" />
						<Button label="Export" icon="pi pi-upload" class="p-button-help" @click="" />
					</template>
				</Toolbar>

				<DataTable ref="dt" :value="clients" v-model:selection="clientsSelected" dataKey="id" :paginator="true" :rows="10" :filters="filters"
							paginatorTemplate="FirstPageLink PrevPageLink PageLinks NextPageLink LastPageLink CurrentPageReport RowsPerPageDropdown" :rowsPerPageOptions="[5,10,25]"
							currentPageReportTemplate="Showing {first} to {last} of {totalRecords} clients" responsiveLayout="scroll">
					<template #header>
						<div class="flex flex-column md:flex-row md:justify-content-between md:align-items-center">
							<h5 class="m-0">Manage Clients</h5>
							<span class="block mt-2 md:mt-0 p-input-icon-left">
                                <i class="pi pi-search" />
                                <InputText v-model="filters['global'].value" placeholder="Search..." />
                            </span>
						</div>
					</template>

					<Column selectionMode="multiple" headerStyle="width: 3rem"></Column>
					<Column field="code" header="Strain ID" headerStyle="width:14%; min-width:10rem;">
						<template #body="slotProps">
							<span class="p-column-title">Strain ID</span>
							{{slotProps.data.code}}
						</template>
					</Column>
					<Column field="name" header="Init Time" headerStyle="width:14%; min-width:10rem;">
						<template #body="slotProps">
							<span class="p-column-title">Init Time</span>
							{{slotProps.data.name}}
						</template>
					</Column>
					<Column header="Modules" headerStyle="width:14%; min-width:10rem;">
						<template #body="slotProps">
							<span class="p-column-title">Modules</span>
							<img :src="'images/product/' + slotProps.data.image" :alt="slotProps.data.image" class="shadow-2" width="100" />
						</template>
					</Column>
					<Column field="price" header="Host OS" headerStyle="width:14%; min-width:8rem;">
						<template #body="slotProps">
							<span class="p-column-title">Host OS</span>
							{{1}}
						</template>
					</Column>
					<Column field="category" header="Host Arch" headerStyle="width:14%; min-width:10rem;">
						<template #body="slotProps">
							<span class="p-column-title">Host Arch</span>
							{{1}}
						</template>
					</Column>
					<Column field="rating" header="Hostname" headerStyle="width:14%; min-width:10rem;">
						<template #body="slotProps">
							<span class="p-column-title">Hostname</span>
							<Rating :modelValue="slotProps.data.rating" :readonly="true" :cancel="false" />
						</template>
					</Column>
					<Column field="inventoryStatus" header="Host User" headerStyle="width:14%; min-width:10rem;">
						<template #body="slotProps">
							<span class="p-column-title">Host User</span>
							<span :class="'product-badge status-' + (slotProps.data.inventoryStatus ? slotProps.data.inventoryStatus.toLowerCase() : '')">{{slotProps.data.inventoryStatus}}</span>
						</template>
					</Column>
					<Column field="inventoryStatus" header="Host User ID" headerStyle="width:14%; min-width:10rem;">
						<template #body="slotProps">
							<span class="p-column-title">Host User ID</span>
							<span :class="'product-badge status-' + (slotProps.data.inventoryStatus ? slotProps.data.inventoryStatus.toLowerCase() : '')">{{slotProps.data.inventoryStatus}}</span>
						</template>
					</Column>
					<Column field="inventoryStatus" header="Errors" headerStyle="width:14%; min-width:10rem;">
						<template #body="slotProps">
							<span class="p-column-title">Errors</span>
							<span :class="'product-badge status-' + (slotProps.data.inventoryStatus ? slotProps.data.inventoryStatus.toLowerCase() : '')">{{slotProps.data.inventoryStatus}}</span>
						</template>
					</Column>
					<Column headerStyle="min-width:10rem;">
						<template #body="slotProps">
							<Button icon="pi pi-pencil" class="p-button-rounded p-button-success mr-2" @click="" />
							<Button icon="pi pi-trash" class="p-button-rounded p-button-warning mt-2" @click="" />
						</template>
					</Column>
				</DataTable>
			</div>
		</div>
	</div>

</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { FilterMatchMode } from 'primevue/api'
import API from '../../api/api'

export default defineComponent({
	data() {
		return {
			api: new API(),
			clients: [],
			clientsSelected: [],
			clientsCount: 0,
			clientsOffset: 0,
			clientsLimit: 50,
			clientDialog: false,
			filters: {
				global: {
					value: '',
					matchMode: FilterMatchMode.CONTAINS,
				}
			},
			timer: 0
		}
	},
	mounted() {
		this.startAutoUpdate()
	},
	beforeUnmount () {
		this.cancelAutoUpdate()
	},
	methods: {
		updatePageData () {
			this.api.fetchClients(this.clientsOffset, this.clientsLimit).then((data) => {
				this.clients = data['clients']
				this.clientsCount = data['total']
			});
		},
		startAutoUpdate() {
			this.updatePageData()
			this.timer = setInterval(this.updatePageData, 5000)
		},
		cancelAutoUpdate() {
			clearInterval(this.timer)
		},
	},
})
</script>

<!-- <script lang="ts">
import {FilterMatchMode} from 'primevue/api';
import ProductService from '../../api/api';

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
		this.productService = new ProductService();
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
		openNew() {
			this.product = {};
			this.submitted = false;
			this.productDialog = true;
		},
		hideDialog() {
			this.productDialog = false;
			this.submitted = false;
		},
		saveProduct() {
			this.submitted = true;
			if (this.product.name.trim()) {
			if (this.product.id) {
				this.product.inventoryStatus = this.product.inventoryStatus.value ? this.product.inventoryStatus.value: this.product.inventoryStatus;
				this.products[this.findIndexById(this.product.id)] = this.product;
				this.$toast.add({severity:'success', summary: 'Successful', detail: 'Product Updated', life: 3000});
				}
				else {
					this.product.id = this.createId();
					this.product.code = this.createId();
					this.product.image = 'product-placeholder.svg';
					this.product.inventoryStatus = this.product.inventoryStatus ? this.product.inventoryStatus.value : 'INSTOCK';
					this.products.push(this.product);
					this.$toast.add({severity:'success', summary: 'Successful', detail: 'Product Created', life: 3000});
				}
				this.productDialog = false;
				this.product = {};
			}
		},
		editProduct(product) {
			this.product = {...product};
			this.productDialog = true;
		},
		confirmDeleteProduct(product) {
			this.product = product;
			this.deleteProductDialog = true;
		},
		deleteProduct() {
			this.products = this.products.filter(val => val.id !== this.product.id);
			this.deleteProductDialog = false;
			this.product = {};
			this.$toast.add({severity:'success', summary: 'Successful', detail: 'Product Deleted', life: 3000});
		},
		findIndexById(id) {
			let index = -1;
			for (let i = 0; i < this.products.length; i++) {
				if (this.products[i].id === id) {
					index = i;
					break;
				}
			}
			return index;
		},
		createId() {
			let id = '';
			var chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
			for ( var i = 0; i < 5; i++ ) {
				id += chars.charAt(Math.floor(Math.random() * chars.length));
			}
			return id;
		},
		exportCSV() {
			this.$refs.dt.exportCSV();
		},
		confirmDeleteSelected() {
			this.deleteProductsDialog = true;
		},
		deleteSelectedProducts() {
			this.products = this.products.filter(val => !this.selectedProducts.includes(val));
			this.deleteProductsDialog = false;
			this.selectedProducts = null;
			this.$toast.add({severity:'success', summary: 'Successful', detail: 'Products Deleted', life: 3000});
		},
		initFilters() {
            this.filters = {
                'global': {value: null, matchMode: FilterMatchMode.CONTAINS},
            }
        }
	}
}
</script> -->

<style scoped lang="scss">
@import '../../assets/styles/badges.scss';
</style>
