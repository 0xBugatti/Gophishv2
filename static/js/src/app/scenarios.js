var scenarios = []
var teams = []
var item_type = "scenarios"

function save(idx) {
    var scenario = {
        name: $("#name").val(),
        description: $("#description").val(),
        url: $("#url").val(),
        page: {
            id: parseInt($("#page").select2("data")[0].id)
        }
    }
    var selected = $("#template").select2("data")
    var templates = []

    for (var i = 0; i < selected.length; i++) {
        templates.push({ id: parseInt($("#template").select2("data")[i].id) })
    }
    scenario["templates"] = templates

    if (idx !== -1) {
        scenario.id = scenarios[idx].id
        api.scenarios.put(scenario)
            .success(function (data) {
                successFlash("Scenario edited successfully!")
                load()
                dismiss()
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    } else {
        // Submit the scenario
        api.scenarios.post(scenario)
            .success(function (data) {
                successFlash("Scenario added successfully!")
                load()
                dismiss()
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    }
}

function dismiss() {
    $("#modal\\.flashes").empty();
    $("#name").val("");
    $("#template").val("").change();
    $("#page").val("").change();
    $("#url").val("");
    $("#users").val("").change();
    $("#modal").modal('hide');
}

function setupScenarioOptions() {
    api.templates.get()
        .success(function (templates) {
            if (templates.length === 0) {
                modalError("No templates found!")
                return false
            } else {
                var template_s2 = $.map(templates, function (obj) {
                    obj.text = obj.name
                    return obj
                });
                var template_select = $("#template.form-control")
                template_select.select2({
                    placeholder: "Select a Template",
                    data: template_s2,
                    width: "100%",
                });
                if (templates.length === 1) {
                    template_select.val(template_s2[0].id)
                    template_select.trigger('change.select2')
                }
            }
        });
    api.pages.get()
        .success(function (pages) {
            if (pages.length === 0) {
                modalError("No pages found!")
                return false
            } else {
                var page_s2 = $.map(pages, function (obj) {
                    obj.text = obj.name
                    return obj
                });
                var page_select = $("#page.form-control")
                page_select.select2({
                    placeholder: "Select a Landing Page",
                    data: page_s2,
                    width: "100%",
                });
                if (pages.length === 1) {
                    page_select.val(page_s2[0].id)
                    page_select.trigger('change.select2')
                }
            }
        });
}

function view(idx) {
    setupScenarioOptions();
    $("#modalSubmit").unbind('click').click(function () {
        save(idx)
    })
    $("#scenarioModalLabel").text("View Scenario")
    scenario = scenarios[idx]
    $("#name").val(scenario.name)
    $("#description").val(scenario.Description)
    $("#url").val(scenario.url)
    var scenarioTemplates = scenario.templates.filter(i => i.hasOwnProperty('id')).map(i => i.id.toString())

    $("#template").val(scenarioTemplates);
    $("#template").trigger("change.select2")

    $("#page").val(scenario.page.id.toString());
    $("#page").trigger("change.select2")
    // make not Editable
    $("#modalSubmit").hide()
    $("#name").attr('readonly', true)
    $("#description").attr('readonly', true)
    $("#url").attr('readonly', true)
}

function edit(idx) {
    $("#name").attr('readonly', false)
    $("#description").attr('readonly', false)
    $("#url").attr('readonly', false)
    setupScenarioOptions()
    $("#modalSubmit").unbind('click').click(function () {
        save(idx)
    })
    if (idx !== -1) {
        $("#scenarioModalLabel").text("Edit Scenario")
        scenario = scenarios[idx]
        $("#name").val(scenario.name)
        $("#description").val(scenario.Description)
        $("#url").val(scenario.url)
        var scenarioTemplates = scenario.templates.filter(i => i.hasOwnProperty('id')).map(i => i.id.toString())

        $("#template").val(scenarioTemplates);
        $("#template").trigger("change.select2")

        $("#page").val(scenario.page.id.toString());
        $("#page").trigger("change.select2")
    } else {
        $("#scenarioModalLabel").text("New Scenario")
    }
}

function copy(idx) {
    $("#modalSubmit").unbind('click').click(function () {
        save(-1)
    })
    $("#name").attr('readonly', false)
    $("#description").attr('readonly', false)
    $("#url").attr('readonly', false)
    setupScenarioOptions();
    // Set our initial values
    api.scenarioId.get(scenarios[idx].id)
        .success(function (scenario) {
            $("#name").val("Copy of " + scenario.name)
            $("#description").val(scenario.Description)
            var scenarioTemplates = scenario.templates.filter(i => i.hasOwnProperty('id')).map(i => i.id.toString())
            
            if (scenarioTemplates.length === 0){
                $("#template").val("").change();
                $("#template").select2({
                    placeholder: "Add Templates"
                });
            } else {
                $('#template').val(scenarioTemplates);
                $("#template").trigger("change.select2")
            }
            if (!scenario.page.hasOwnProperty('id')) {
                $("#page").val("").change();
                $("#page").select2({
                    placeholder: scenario.page.name
                });
            } else {
                $("#page").val(scenario.page.id.toString());
                $("#page").trigger("change.select2")
            }
            $("#url").val(scenario.url)
        })
        .error(function (data) {
            $("#modal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
            <i class=\"fa fa-exclamation-circle\"></i> " + data.responseJSON.message + "</div>")
        })
}

var deleteScenario = function (idx) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the scenario. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + escapeHtml(scenarios[idx].name),
        confirmButtonColor: "#10B981",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.scenarios.delete(scenarios[idx].id)
                    .success(function (msg) {
                        resolve()
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            }).catch(function (error) {
                Swal.showValidationMessage(
                    `Request failed: ${error}`
                );
                return false;
            })
        }
    }).then(function (result) {
        if (result.value){
            Swal.fire(
                'Scenario Deleted!',
                'This scenario has been deleted!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            location.reload()
        })
    })
}

function load() {
    $("#scenarioTable").hide()
    $("#emptyMessage").hide()
    $("#loading").show()
    api.scenarios.get()
        .success(function (ss) {
            api.user.current().success(function (u) {
                teams = u.teams
                scenarios = ss
                $("#loading").hide()
                if (scenarios.length > 0) {
                    $("#scenarioTable").show()
                    scenarioTable = $("#scenarioTable").DataTable({
                        destroy: true,
                        columnDefs: [{
                            orderable: false,
                            targets: "no-sort"
                        }]
                    });
                    scenarioTable.clear()
                    scenarioRows = []
                    scenarios = scenarios.filter((item, index, self) => 
                    index === self.findIndex((t) => t.id === item.id)
                    );
                    
                    $.each(scenarios, function (i, scenario) {
                        var permissions = {
                            canDelete : false,
                            canEdit : false,
                        };
                        permissions = CheckTeam(scenario.teams, u)

                        var isOwner = false;
                        if (u.id == scenario.user_id){
                            var isOwner = true;
                        }
                        scenarioRows.push([
                            escapeHtml(scenario.name),
                            moment(scenario.modified_date).format('MMMM Do YYYY, h:mm:ss a'),
                            "<div class='pull-right'>\
                                <span data-toggle='modal' data-backdrop='static' data-target='#modal'>\
                                " + (isOwner || permissions.canEdit ? "<button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Edit Scenarios' onclick='edit("+ i + ")')>\
                                <i class='fa fa-pencil'></i>\
                                </button>" :
                                "<button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='View Scenarios' onclick='view("+ i + ")')></i>\
                                <i class='fa fa-eye'></i>\
                                </button>") + "\
                                </span>\
                                <span data-toggle='modal' data-target='#modal'>\
                                    <button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Copy Scenarios' onclick='copy(" + i + ")'>\
                                        <i class='fa fa-copy'></i>\
                                    </button>\
                                </span>\
                                <button class='btn " + (isOwner || permissions.canDelete ? "btn-danger" : "btn-secondary disabled") + "' data-toggle='tooltip' data-placement='left' title='" + (isOwner || permissions.canDelete ? "Delete Page" : "You dont have permission to delete this scenario") + "' " + (isOwner || permissions.canDelete ? "onclick='deleteScenario(" + i + ")'" : "disabled") + ">\
                                    <i class='fa fa-trash-o'></i>\
                                </button>\
                                <button id='teams_button' type='button' class='btn "+ (isOwner ? "btn-orange" : "btn-secondary disabled") + "' data-toggle='modal' data-backdrop='static' data-target='#team_modal'" + (isOwner ? "onclick='team("+ i + ")'" : "disabled") + "'>\
                                <i class='fa fa-users'></i> Update teams\
                                </button>\
                            </div>"
                        ])
                    })
                    scenarioTable.rows.add(scenarioRows).draw()
                    $('[data-toggle="tooltip"]').tooltip()
                } else {
                    $("#emptyMessage").show()
                }
            })
        })
        .error(function () {
            $("#loading").hide()
            errorFlash("Error fetching scenarios")
        })
}

$(document).ready(function () {
    // Setup multiple modals
    // Code based on http://miles-by-motorcycle.com/static/bootstrap-modal/index.html
    $('.modal').on('hidden.bs.modal', function (event) {
        $(this).removeClass('fv-modal-stack');
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') - 1);
    });
    $('.modal').on('shown.bs.modal', function (event) {
        // Keep track of the number of open modals
        if (typeof ($('body').data('fv_open_modals')) == 'undefined') {
            $('body').data('fv_open_modals', 0);
        }
        // if the z-index of this modal has been set, ignore.
        if ($(this).hasClass('fv-modal-stack')) {
            return;
        }
        $(this).addClass('fv-modal-stack');
        // Increment the number of open modals
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') + 1);
        // Setup the appropriate z-index
        $(this).css('z-index', 1040 + (10 * $('body').data('fv_open_modals')));
        $('.modal-backdrop').not('.fv-modal-stack').css('z-index', 1039 + (10 * $('body').data('fv_open_modals')));
        $('.modal-backdrop').not('fv-modal-stack').addClass('fv-modal-stack');
    });
    $.fn.modal.Constructor.prototype.enforceFocus = function () {
        $(document)
            .off('focusin.bs.modal') // guard against infinite focus loop
            .on('focusin.bs.modal', $.proxy(function (e) {
                if (
                    this.$element[0] !== e.target && !this.$element.has(e.target).length
                    // CKEditor compatibility fix start.
                    &&
                    !$(e.target).closest('.cke_dialog, .cke').length
                    // CKEditor compatibility fix end.
                ) {
                    this.$element.trigger('focus');
                }
            }, this));
    };
    // Scrollbar fix - https://stackoverflow.com/questions/19305821/multiple-modals-overlay
    $(document).on('hidden.bs.modal', '.modal', function () {
        $('.modal:visible').length && $(document.body).addClass('modal-open');
    });
    $('#modal').on('hidden.bs.modal', function (event) {
        dismiss()
    });
    load()

})

function team(i) {
    updateItemTeamsAssignment(scenarios[i], item_type, teams)
}