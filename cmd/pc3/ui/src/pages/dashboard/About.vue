<template>
	<div class="grid">
		<div class="col-12">
			<div class="card">
				<h5>About</h5>
				<p>Wraith Module PineComms Command & Control (pc3) Panel</p>

				<BlockViewer header="Build Information" :code="aboutBuild"></BlockViewer>
			</div>
		</div>
	</div>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import API from '../../api/api'

export default defineComponent({
	data() {
		return {
			api: new API(),
			aboutBuild: 'no build info',
		}
	},
	mounted() {
		this.api.fetchAbout().then((data) => {
			this.aboutBuild = JSON.stringify(data['build'], undefined, 4)
		});
	},
})
</script>