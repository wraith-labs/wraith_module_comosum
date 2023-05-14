<template>
	<div class="grid">
		<div class="col-12">
			<div class="card">
				<Toast/>
				<Toolbar class="mb-4">
					<template v-slot:start>
						<div class="my-2">
							<Button label="Send To All" icon="pi pi-send" class="p-button-success mr-2" @click="selectTargets('all')" />
							<Button
								label="Send To Selected"
								icon="pi pi-send"
								class="p-button-success mr-2"
								@click="selectTargets(clientsSelected.map((client) => client.address))"
								:disabled="!clientsSelected || !clientsSelected.length" />
						</div>
					</template>
				</Toolbar>

				<DataTable ref="dt" :value="clients" v-model:selection="clientsSelected" dataKey="address" :paginator="true" :rows="clientsCount" :filters="filters"
							paginatorTemplate="FirstPageLink PrevPageLink PageLinks NextPageLink LastPageLink CurrentPageReport RowsPerPageDropdown" :rowsPerPageOptions="[10, 25, 50, 100]"
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
					<Column headerStyle="min-width:3rem;">
						<template #body="slotProps">
							<Button icon="pi pi-send" class="p-button-rounded p-button-success mr-2" @click="selectTargets([slotProps.data.address])" />
						</template>
					</Column>
					<Column field="address" header="Address" headerStyle="width:14%; min-width:10rem;">
						<template #body="slotProps">
							<span class="p-column-title">Address</span>
							{{slotProps.data.address}}
						</template>
					</Column>
					<Column field="strain" header="Strain ID" headerStyle="width:14%; min-width:10rem;">
						<template #body="slotProps">
							<span class="p-column-title">Strain ID</span>
							{{slotProps.data.lastHeartbeat.StrainId}}
						</template>
					</Column>
					<Column field="inittime" header="Init Time" headerStyle="width:14%; min-width:10rem;">
						<template #body="slotProps">
							<span class="p-column-title">Init Time</span>
							{{slotProps.data.lastHeartbeat.InitTime}}
						</template>
					</Column>
					<Column field="modules" header="Modules" headerStyle="width:14%; min-width:10rem;">
						<template #body="slotProps">
							<span class="p-column-title">Modules</span>
							{{slotProps.data.lastHeartbeat.Modules}}
						</template>
					</Column>
					<Column field="hostname" header="Hostname" headerStyle="width:14%; min-width:10rem;">
						<template #body="slotProps">
							<span class="p-column-title">Hostname</span>
							{{slotProps.data.lastHeartbeat.Hostname}}
						</template>
					</Column>
					<Column field="hostos" header="Host OS" headerStyle="width:14%; min-width:8rem;">
						<template #body="slotProps">
							<span class="p-column-title">Host OS</span>
							{{slotProps.data.lastHeartbeat.HostOS}}
						</template>
					</Column>
					<Column field="hostarch" header="Host Arch" headerStyle="width:14%; min-width:10rem;">
						<template #body="slotProps">
							<span class="p-column-title">Host Arch</span>
							{{slotProps.data.lastHeartbeat.HostArch}}
						</template>
					</Column>
					<Column field="hostuser" header="Host User" headerStyle="width:14%; min-width:10rem;">
						<template #body="slotProps">
							<span class="p-column-title">Host User</span>
							{{slotProps.data.lastHeartbeat.HostUser}}
						</template>
					</Column>
					<Column field="hostuid" header="Host User ID" headerStyle="width:14%; min-width:10rem;">
						<template #body="slotProps">
							<span class="p-column-title">Host User ID</span>
							{{slotProps.data.lastHeartbeat.HostUserId}}
						</template>
					</Column>
					<Column field="errors" header="Errors" headerStyle="width:14%; min-width:10rem;">
						<template #body="slotProps">
							<span class="p-column-title">Errors</span>
							{{slotProps.data.lastHeartbeat.Errors}}
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
import { store } from '../../state/state'
import { Client } from '../../api/types'

export default defineComponent({
	data() {
		return {
			api: new API(),
			store,
			clients: [] as Client[],
			clientsSelected: []as Client[],
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
			timer: 0,
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
		selectTargets(targets: string[] | 'all') {
			store.targets = targets
			window.location.hash = '#/clients/console'
		}
	},
})
</script>

<style scoped lang="scss">
@import '../../assets/styles/badges.scss';
</style>
