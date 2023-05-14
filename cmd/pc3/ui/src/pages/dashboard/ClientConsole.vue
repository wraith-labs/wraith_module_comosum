<template>
	<div class="grid">
		<div class="col-12">
			<div class="card">
				<h5>Console Page</h5>
				<p>{{ store.targets }}</p>
			</div>
		</div>
	</div>
</template>

<script lang="ts">
import { defineComponent } from 'vue';
import { store } from '../../state/state'

export default defineComponent({
	data() {
		return {
			store
		}
	}
})
</script>