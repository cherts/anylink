<template>
    <div>
        <el-card>

            <el-form :inline="true">
                <el-form-item>
                    <el-button
                            size="small"
                            type="primary"
                            icon="el-icon-plus"
                            @click="handleEdit('')">Add
                    </el-button>
                </el-form-item>
                <!--
                <el-form-item>
                    <el-alert
                            title="After directly operating the database to add, delete or modify data, please restart the anylink service."
                            type="warning">
                    </el-alert>
                </el-form-item>
                -->
            </el-form>

            <el-table
                    ref="multipleTable"
                    :data="tableData"
                    border>

                <el-table-column
                        sortable="true"
                        prop="id"
                        label="ID"
                        width="60">
                </el-table-column>

                <el-table-column
                        prop="ip_addr"
                        label="IP address">
                </el-table-column>

                <el-table-column
                        prop="mac_addr"
                        label="MAC address">
                </el-table-column>

                <el-table-column
                        prop="unique_mac"
                        label="Unique MAC">
                    <template slot-scope="scope">
                        <el-tag v-if="scope.row.unique_mac" type="success">Yes</el-tag>
                        <el-tag v-else type="info">No</el-tag>
                    </template>
                </el-table-column>

                <el-table-column
                        prop="username"
                        label="Username">
                </el-table-column>

                <el-table-column
                        prop="keep"
                        label="IP reservation">
                    <template slot-scope="scope">
                        <!--<el-tag v-if="scope.row.keep" type="success">Reserve</el-tag>-->
                        <el-switch
                                disabled
                                v-model="scope.row.keep"
                                active-color="#13ce66">
                        </el-switch>
                    </template>
                </el-table-column>

                <el-table-column
                    prop="keep_username"
                    label="IP reservation (Username)">
                    <template slot-scope="scope">
                        <el-switch
                            disabled
                            v-model="scope.row.keep_username"
                            active-color="#13ce66">
                        </el-switch>
                    </template>
                </el-table-column>

                <el-table-column
                        prop="note"
                        label="Description">
                </el-table-column>

                <el-table-column
                        prop="last_login"
                        label="Last login time"
                        :formatter="tableDateFormat">
                </el-table-column>

                <el-table-column
                        label="Actions"
                        width="200">
                    <template slot-scope="scope">
                        <el-button
                                size="mini"
                                type="primary"
                                @click="handleEdit(scope.row)">Edit
                        </el-button>

                        <el-popconfirm
                                class="m-left-10"
                                @confirm="handleDel(scope.row)"
                                title="Are you sure you want to delete the IP mapping?">
                            <el-button
                                    slot="reference"
                                    size="mini"
                                    type="danger">Delete
                            </el-button>
                        </el-popconfirm>

                    </template>
                </el-table-column>
            </el-table>

            <div class="sh-20"></div>

            <el-pagination
                    background
                    layout="prev, pager, next"
                    :pager-count="11"
                    @current-change="pageChange"
                    :total="count">
            </el-pagination>

        </el-card>

        <!--Add and modify pop-up boxes-->
        <el-dialog
                title="Adding IP mapping"
                :close-on-click-modal="false"
                :visible="user_edit_dialog"
                @close="disVisible"
                width="650px"
                center>

            <el-form :model="ruleForm" :rules="rules" ref="ruleForm" label-width="100px" class="ruleForm">
                <el-form-item label="ID" prop="id">
                    <el-input v-model="ruleForm.id" disabled></el-input>
                </el-form-item>
                <el-form-item label="IP address" prop="ip_addr">
                    <el-input v-model="ruleForm.ip_addr"></el-input>
                </el-form-item>
                <el-form-item label="MAC address" prop="mac_addr">
                    <el-input v-model="ruleForm.mac_addr"></el-input>
                </el-form-item>
                <el-form-item label="Username" prop="username">
                    <el-input v-model="ruleForm.username"></el-input>
                </el-form-item>

                <el-form-item label="Description" prop="note">
                    <el-input v-model="ruleForm.note"></el-input>
                </el-form-item>

                <el-form-item label="IP reservation" prop="keep">
                    <el-switch
                            v-model="ruleForm.keep"
                            @change="onKeepChange"
                            active-color="#13ce66">
                    </el-switch>
                </el-form-item>

                <el-form-item label="IP reservation (Username)" prop="keep_username">
                    <el-switch
                        v-model="ruleForm.keep_username"
                        @change="onKeepUsernameChange"
                        active-color="#13ce66">
                    </el-switch>
                </el-form-item>


                <el-form-item>
                    <el-button type="primary" @click="submitForm('ruleForm')">Save</el-button>
                    <el-button @click="disVisible">Cancel</el-button>
                </el-form-item>
            </el-form>

        </el-dialog>

    </div>
</template>

<script>
import axios from "axios";

export default {
    name: "IpMap",
    components: {},
    mixins: [],
    created() {
        this.$emit('update:route_path', this.$route.path)
        this.$emit('update:route_name', ['Users', 'IP mapping'])
    },
    mounted() {
        this.getData(1)
    },   computed: {
        rules() {
            return {
                username: [
                    {required: !!this.ruleForm.keep_username, message: 'Please enter username', trigger: 'blur'},
                    {max: 50, message: 'Less than 50 characters long', trigger: 'blur'}
                ],
                mac_addr: [
                    {required: !this.ruleForm.keep_username, message: 'Please enter mac address', trigger: 'blur'}
                ],
                ip_addr: [
                    {required: true, message: 'Please enter ip address', trigger: 'blur'}
                ],
                status: [
                    {required: true}
                ],
            }
        }
    },
    data() {
        return {
            tableData: [],
            count: 10,
            nowIndex: 0,
            ruleForm: {
                status: 1,
                groups: [],
                keep: false,
                keep_username: false,
            },
        }
    },
    methods: {
        onKeepChange(val) {
            if (val) {
                this.ruleForm.keep_username = false;
            }
        },
        onKeepUsernameChange(val) {
            if (val) {
                this.ruleForm.keep = false;
            }
            if (this.$refs['ruleForm']) {
                this.$refs['ruleForm'].clearValidate(['mac_addr', 'username']);
            }
        },

        getData(p) {
            axios.get('/user/ip_map/list', {
                params: {
                    page: p,
                }
            }).then(resp => {
                var data = resp.data.data
                //console.log(data);
                this.tableData = data.datas;
                this.count = data.count
            }).catch(error => {
                this.$message.error('Oh, request error');
                console.log(error);
            });
        },
        pageChange(p) {
            this.getData(p)
        },
        handleEdit(row) {
            !this.$refs['ruleForm'] || this.$refs['ruleForm'].resetFields();
            this.user_edit_dialog = true
            if (!row) {
                this.ruleForm = {
                    status: 1,
                    groups: [],
                    keep: false,
                    keep_username: false,
                };
                return;
            }

            axios.get('/user/ip_map/detail', {
                params: {
                    id: row.id,
                }
            }).then(resp => {
                this.ruleForm = resp.data.data
            }).catch(error => {
                this.$message.error('Oh, request error');
                console.log(error);
            });
        },
        handleDel(row) {
            axios.post('/user/ip_map/del?id=' + row.id).then(resp => {
                var rdata = resp.data
                if (rdata.code === 0) {
                    this.$message.success(rdata.msg);
                    this.getData(1);
                } else {
                    this.$message.error(rdata.msg);
                }
                //console.log(rdata);
            }).catch(error => {
                this.$message.error('Oh, request error');
                console.log(error);
            });
        },
        submitForm(formName) {
            this.$refs[formName].validate((valid) => {
                if (!valid) {
                    //console.log('Error submit!');
                    return false;
                }

                // alert('submit!');
                axios.post('/user/ip_map/set', this.ruleForm).then(resp => {
                    var rdata = resp.data
                    if (rdata.code === 0) {
                        this.$message.success(rdata.msg);
                        this.getData(1);
                    } else {
                        this.$message.error(rdata.msg);
                    }
                    //console.log(rdata);
                }).catch(error => {
                    this.$message.error('Oh, request error');
                    console.log(error);
                });
            });
        },
    },
}
</script>

<style scoped>

</style>
