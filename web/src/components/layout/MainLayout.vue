<template>
  <div class="main-layout">
    <el-container>
      <el-header>
        <h1>Robot Remote Maintenance</h1>
      </el-header>
      <el-container>
        <el-aside width="200px">
          <el-menu :default-active="$route.path" router>
            <el-menu-item index="/">Dashboard</el-menu-item>
            <el-menu-item index="/devices">Devices</el-menu-item>
            <el-menu-item index="/audit">Audit Logs</el-menu-item>
          </el-menu>
        </el-aside>
        <el-main>
          <router-view />
        </el-main>
      </el-container>
    </el-container>
  </div>
</template>

<script setup lang="ts">
</script>

<style scoped>
.main-layout {
  height: 100vh;
}

.el-header {
  background-color: #001529;
  color: #fff;
  display: flex;
  align-items: center;
}

.el-header h1 {
  margin: 0;
  font-size: 18px;
}
</style>
